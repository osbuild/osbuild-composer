// Mock dnf-json
//
// The purpose of this program is to return fake but expected responses to
// dnf-json depsolve and dump queries.  Tests should initialise a
// dnfjson.Solver and configure it to run this program via the SetDNFJSONPath()
// method.  This utility accepts queries and returns responses with the same
// structure as the dnf-json Python script.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/osbuild/osbuild-composer/internal/dnfjson"
)

func maybeFail(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func readRequest(r io.Reader) dnfjson.Request {
	j := json.NewDecoder(os.Stdin)
	j.DisallowUnknownFields()

	var req dnfjson.Request
	err := j.Decode(&req)
	maybeFail(err)
	return req
}

func readTestCase() string {
	if len(os.Args) < 2 {
		fail(errors.New("no test case specified"))
	}
	if len(os.Args) > 2 {
		fail(errors.New("invalid number of arguments: you must specify a test case"))
	}
	return os.Args[1]
}

func parseResponse(resp []byte, command string) json.RawMessage {
	parsedResponse := make(map[string]json.RawMessage)
	err := json.Unmarshal(resp, &parsedResponse)
	maybeFail(err)
	if command == "chain-depsolve" {
		// treat chain-depsolve and depsolve the same
		command = "depsolve"
	}
	return parsedResponse[command]
}

func checkForError(msg json.RawMessage) bool {
	j := json.NewDecoder(bytes.NewReader(msg))
	j.DisallowUnknownFields()
	dnferror := new(dnfjson.Error)
	err := j.Decode(dnferror)
	return err == nil
}

func main() {
	testFilePath := readTestCase()

	req := readRequest(os.Stdin)

	testFile, err := os.Open(testFilePath)
	if err != nil {
		fail(fmt.Errorf("failed to open test file %q\n", testFilePath))
	}
	defer testFile.Close()
	response, err := io.ReadAll(testFile)
	if err != nil {
		fail(fmt.Errorf("failed to read test file %q\n", testFilePath))
	}

	res := parseResponse(response, req.Command)
	fmt.Print(string(parseResponse(response, req.Command)))

	// check if we should return with error
	if checkForError(res) {
		os.Exit(1)
	}
}
