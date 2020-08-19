// Copyright 2015 Andrew E. Bruno
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package khttp is a transport that authenticates all outgoing requests using
// SPNEGO (negotiate authentication) http://tools.ietf.org/html/rfc4559.
package khttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ubccr/kerby"
)

var (
	negotiateHeader       = "Negotiate"
	wwwAuthenticateHeader = "WWW-Authenticate"
	authorizationHeader   = "Authorization"
)

// HTTP client transport that authenticates all outgoing
// requests using SPNEGO. Implements the http.RoundTripper interface
type Transport struct {
	// keytab file to use
	KeyTab string
	// principal
	Principal string
	// Next specifies the next transport to be used or http.DefaultTransport if nil.
	Next http.RoundTripper
}

// RoundTrip executes a single HTTP transaction performing SPNEGO negotiate
// authentication.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.KeyTab) > 0 {
		os.Setenv("KRB5_CLIENT_KTNAME", t.KeyTab)
	}
	host, _, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		host = req.URL.Host
	}
	service := fmt.Sprintf("HTTP@%s", host)
	kc := new(kerby.KerbClient)
	err = kc.Init(service, t.Principal)
	if err != nil {
		return nil, err
	}
	defer kc.Clean()

	err = kc.Step("")
	if err != nil {
		return nil, err
	}

	req.Header.Set(authorizationHeader, negotiateHeader+" "+kc.Response())

	tr := t.Next
	if tr == nil {
		tr = http.DefaultTransport
		if tr == nil {
			return nil, errors.New("khttp: no Next transport or DefaultTransport")
		}
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	authReply := strings.Split(resp.Header.Get(wwwAuthenticateHeader), " ")
	if len(authReply) != 2 || strings.ToLower(authReply[0]) != strings.ToLower(negotiateHeader) {
		return nil, errors.New("khttp: server replied with invalid www-authenticate header")
	}

	// Authenticate the reply from the server
	err = kc.Step(authReply[1])
	if err != nil {
		return nil, err
	}

	return resp, nil
}
