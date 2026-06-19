package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// Inspect a target ref, either local or remote, returning its manifest. Uses
// 'skopeo'.
func (cl *Client) skopeoInspect(target string) ([]byte, error) {
	cmd := exec.Command("skopeo", "inspect", "--raw")

	if tls := cl.GetTLSVerify(); tls != nil && !*tls {
		cmd.Args = append(cmd.Args, "--tls-verify=false")
	}

	if authfile := cl.GetAuthFilePath(); authfile != "" {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--authfile=%s", authfile))
	}

	cmd.Args = append(cmd.Args, target)

	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout

	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}
