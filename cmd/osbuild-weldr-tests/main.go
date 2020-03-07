// osbuild-tests runs all of the osbuild integration tests against a live server
// Copyright (C) 2020 by Red Hat, Inc.
package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/osbuild/osbuild-composer/internal/weldrcheck"
)

func main() {
	client := &http.Client{
		// TODO This may be too short/simple for downloading images
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/weldr/api.socket")
			},
		},
	}

	weldrcheck.Run(client)
}
