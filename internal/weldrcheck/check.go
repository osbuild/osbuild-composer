// Package weldrcheck contains functions used to run integration tests on a running API server
// Copyright (C) 2020 by Red Hat, Inc.
package weldrcheck

import (
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/client"
)

// Checks should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run checks that depend on previous results from the same function
// not from other functions.
// The blueprint version number may get bumped if the server has had tests run before
// do not assume the bp version will match unless first deleting the old one.

// isStringInSlice returns true if the string is present, false if not
// slice must be sorted
// TODO decide if this belongs in a more widely useful package location
func isStringInSlice(slice []string, s string) bool {
	i := sort.SearchStrings(slice, s)
	if i < len(slice) && slice[i] == s {
		return true
	}
	return false
}

// Run the API V0 checks against the server
// Return true if all the checks pass
func runV0Checks(socket *http.Client) (pass bool) {
	pass = true

	bpv0 := checkBlueprintsV0{socket}
	pass = bpv0.Run()

	if pass {
		log.Println("OK: ALL V0 API checks were successful")
	} else {
		log.Println("FAIL: One or more V0 API checks failed")
	}
	return pass
}

// Run the V1 checks against the server
func runV1Checks(socket *http.Client) (pass bool) {
	pass = true

	if pass {
		log.Println("OK: ALL V1 API checks were successful")
	} else {
		log.Println("FAIL: One or more V1 API checks failed")
	}
	return pass
}

// Run executes all of the weldr API checks against a running API server
// This is designed to run against any WELDR API server, not just osbuild-composer
func Run(socket *http.Client) {
	log.Print("Running API check")

	// Does the server respond to /api/status?
	status, resp, err := client.GetStatusV0(socket)
	if err != nil {
		log.Printf("ERROR: status request failed with client error: %s", err)
		// If status check fails there is no point in continuing
		os.Exit(1)
	}
	if resp != nil {
		log.Printf("ERROR: status request failed: %v", resp)
		// If status check fails there is no point in continuing
		os.Exit(1)
	}
	log.Print("OK: status request")
	apiVersion, e := strconv.Atoi(status.API)
	if e != nil {
		log.Printf("ERROR: status API version error: %s", e)
		log.Println("ERROR: Only running V0 checks")
		apiVersion = 0
	}
	log.Printf("INFO: Running tests against: %s %s server using V%d API", status.Backend, status.Build, apiVersion)

	// Run the V0 checks
	log.Println("INFO: Running API V0 checks")
	pass := runV0Checks(socket)

	// Run the V1 checks if the server claims to support it
	if apiVersion > 0 {
		log.Println("INFO: Running API V1 checks")
		passV1 := runV1Checks(socket)

		pass = pass && passV1
	}

	if !pass {
		os.Exit(1)
	}
	os.Exit(0)
}
