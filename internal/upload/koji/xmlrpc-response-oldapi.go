// +build kolo_xmlrpc_oldapi
//
// This file provides a wrapper around kolo/xmlrpc response handling.
//
// Commit e3ad6d89 of the xmlrpc library changed the API of response handling.
// This means that different APIs are available in Fedora 32 and 33 (it does
// not matter for RHEL as uses vendored libraries).
// This wrapper allows us to use both xmlrpc's APIs using buildflags.
//
// This file is a wrapper for xmlrpc older than e3ad6d89.

package koji

import (
	"fmt"

	"github.com/kolo/xmlrpc"
)

// processXMLRPCResponse is a wrapper around kolo/xmlrpc
func processXMLRPCResponse(body []byte, reply interface{}) error {
	resp := xmlrpc.NewResponse(body)
	if resp.Failed() {
		return fmt.Errorf("xmlrpc server returned an error: %v", resp.Err())
	}

	err := resp.Unmarshal(reply)
	if err != nil {
		return fmt.Errorf("cannot unmarshal the xmlrpc response: %v", err)
	}

	return nil
}
