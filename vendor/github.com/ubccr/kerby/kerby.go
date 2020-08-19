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

// Package kerby is a cgo wrapper for Kerberos GSSAPI
package kerby

/*
#cgo CFLAGS: -std=gnu99
#cgo LDFLAGS: -lgssapi_krb5 -lkrb5 -lk5crypto -lcom_err
#include "kerberosgss.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

// Kerberos GSSAPI Client
type KerbClient struct {
	state *C.gss_client_state
}

// Kerberos GSSAPI Server
type KerbServer struct {
	state *C.gss_server_state
}

// Returns the last major/minor GSSAPI error messages
func (kc KerbClient) GssError() error {
	bufMaj := (*C.char)(C.calloc(C.GSS_ERRBUF_SIZE, 1))
	bufMin := (*C.char)(C.calloc(C.GSS_ERRBUF_SIZE, 1))
	defer C.free(unsafe.Pointer(bufMaj))
	defer C.free(unsafe.Pointer(bufMin))

	C.get_gss_error(kc.state.maj_stat, bufMaj, kc.state.min_stat, bufMin)
	return errors.New(C.GoString(bufMaj) + " - " + C.GoString(bufMin))
}

// Initializes a context for Kerberos GSSAPI client-side authentication.
// KerbClient.Clean must be called after this function returns succesfully to
// dispose of the context once all GSSAPI operations are complete. srv is the
// service principal in the form "type@fqdn". princ is the client principal in the
// form "user@realm".
func (kc *KerbClient) Init(srv, princ string) error {
	service := C.CString(srv)
	defer C.free(unsafe.Pointer(service))
	principal := C.CString(princ)
	defer C.free(unsafe.Pointer(principal))

	var delegatestate *C.gss_server_state
	gss_flags := C.long(C.GSS_C_MUTUAL_FLAG | C.GSS_C_SEQUENCE_FLAG)
	result := 0

	kc.state = C.new_gss_client_state()
	if kc.state == nil {
		return errors.New("Failed to allocate memory for gss_client_state")
	}

	result = int(C.authenticate_gss_client_init(service, principal, gss_flags, delegatestate, kc.state))

	if result == C.AUTH_GSS_ERROR {
		return kc.GssError()
	}

	return nil
}

// Get the client response from the last successful GSSAPI client-side step.
func (kc *KerbClient) Response() string {
	return C.GoString(kc.state.response)
}

// Processes a single GSSAPI client-side step using the supplied server data.
func (kc *KerbClient) Step(chlg string) error {
	challenge := C.CString(chlg)
	defer C.free(unsafe.Pointer(challenge))
	result := 0

	if kc.state == nil {
		return errors.New("Invalid client state")
	}

	result = int(C.authenticate_gss_client_step(kc.state, challenge))

	if result == C.AUTH_GSS_ERROR {
		return kc.GssError()
	}

	return nil
}

// Destroys the context for GSSAPI client-side authentication. After this call
// the KerbClient.state object is invalid and should not be used again.
func (kc *KerbClient) Clean() {
	if kc.state != nil {
		C.authenticate_gss_client_clean(kc.state)
		C.free_gss_client_state(kc.state)
		kc.state = nil
	}
}

// Returns the service principal for the server given a service type and
// hostname. Adopted from PyKerberos.
func ServerPrincipalDetails(service, hostname string) (string, error) {
	var code C.krb5_error_code
	var kcontext C.krb5_context
	var kt C.krb5_keytab
	var cursor C.krb5_kt_cursor
	var entry C.krb5_keytab_entry
	var pname *C.char

	match := fmt.Sprintf("%s/%s@", service, hostname)

	code = C.krb5_init_context(&kcontext)
	if code != 0 {
		return "", fmt.Errorf("Cannot initialize Kerberos5 context: %d", code)
	}

	code = C.krb5_kt_default(kcontext, &kt)
	if code != 0 {
		return "", fmt.Errorf("Cannot get default keytab: %d", int(code))
	}

	code = C.krb5_kt_start_seq_get(kcontext, kt, &cursor)
	if code != 0 {
		return "", fmt.Errorf("Cannot get sequence cursor from keytab: %d", int(code))
	}

	result := ""
	for {
		code = C.krb5_kt_next_entry(kcontext, kt, &entry, &cursor)
		if code != 0 {
			break
		}

		code = C.krb5_unparse_name(kcontext, entry.principal, &pname)
		if code != 0 {
			return "", fmt.Errorf("Cannot parse principal name from keytab: %d", int(code))
		}

		result = C.GoString(pname)
		if strings.HasPrefix(result, match) {
			C.krb5_free_unparsed_name(kcontext, pname)
			C.krb5_free_keytab_entry_contents(kcontext, &entry)
			break
		}

		result = ""
		C.krb5_free_unparsed_name(kcontext, pname)
		C.krb5_free_keytab_entry_contents(kcontext, &entry)
	}

	if len(result) == 0 {
		return "", errors.New("Principal not found in keytab")
	}

	if cursor != nil {
		C.krb5_kt_end_seq_get(kcontext, kt, &cursor)
	}

	if kt != nil {
		C.krb5_kt_close(kcontext, kt)
	}

	C.krb5_free_context(kcontext)

	return result, nil
}

// Returns the last major/minor GSSAPI error messages
func (ks KerbServer) GssError() error {
	bufMaj := (*C.char)(C.calloc(C.GSS_ERRBUF_SIZE, 1))
	bufMin := (*C.char)(C.calloc(C.GSS_ERRBUF_SIZE, 1))
	defer C.free(unsafe.Pointer(bufMaj))
	defer C.free(unsafe.Pointer(bufMin))

	C.get_gss_error(ks.state.maj_stat, bufMaj, ks.state.min_stat, bufMin)
	return errors.New(C.GoString(bufMaj) + " - " + C.GoString(bufMin))
}

// Initializes a context for GSSAPI server-side authentication with the given
// service principal. KerbServer.Clean must be called after this function
// returns succesfully to dispose of the context once all GSSAPI operations are
// complete. srv is the service principal in the form "type@fqdn".
func (ks *KerbServer) Init(srv string) error {
	service := C.CString(srv)
	defer C.free(unsafe.Pointer(service))

	result := 0

	ks.state = C.new_gss_server_state()
	if ks.state == nil {
		return errors.New("Failed to allocate memory for gss_server_state")
	}

	result = int(C.authenticate_gss_server_init(service, ks.state))

	if result == C.AUTH_GSS_ERROR {
		return ks.GssError()
	}

	return nil
}

// Get the user name of the principal trying to authenticate to the server.
// This method must only be called after KerbServer.Step returns a complete or
// continue response code.
func (ks *KerbServer) UserName() string {
	return C.GoString(ks.state.username)
}

// Get the target name if the server did not supply its own credentials.  This
// method must only be called after KerbServer.Step returns a complete or
// continue response code.
func (ks *KerbServer) TargetName() string {
	return C.GoString(ks.state.targetname)
}

// Get the server response from the last successful GSSAPI server-side step.
func (ks *KerbServer) Response() string {
	return C.GoString(ks.state.response)
}

// Processes a single GSSAPI server-side step using the supplied client data.
func (ks *KerbServer) Step(chlg string) error {
	challenge := C.CString(chlg)
	defer C.free(unsafe.Pointer(challenge))
	result := 0

	if ks.state == nil {
		return errors.New("Invalid client state")
	}

	result = int(C.authenticate_gss_server_step(ks.state, challenge))

	if result == C.AUTH_GSS_ERROR {
		return ks.GssError()
	}

	return nil
}

// Destroys the context for GSSAPI server-side authentication. After this call
// the KerbServer.state object is invalid and should not be used again.
func (ks *KerbServer) Clean() {
	if ks.state != nil {
		C.authenticate_gss_server_clean(ks.state)
		C.free_gss_server_state(ks.state)
		ks.state = nil
	}
}
