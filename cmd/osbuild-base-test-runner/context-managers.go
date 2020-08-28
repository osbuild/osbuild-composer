// +build integration

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

func withTempDir(dir, pattern string, f func(dir string) error) error {
	tempDir, err := ioutil.TempDir(dir, pattern)
	if err != nil {
		return fmt.Errorf("cannot create the temporary directory %#v", err)
	}

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			log.Printf("cannot remove the temporary directory: %#v", err)
		}
	}()

	return f(tempDir)
}

// withSSHKeyPair runs the function f with a newly generated
// ssh key-pair, they key-pair is deleted immediately after
// the function f returns
func withSSHKeyPair(f func(privateKey, publicKey string) error) error {
	return withTempDir("", "keys", func(dir string) error {
		privateKey := dir + "/id_rsa"
		publicKey := dir + "/id_rsa.pub"
		cmd := exec.Command("ssh-keygen",
			"-N", "",
			"-f", privateKey,
		)

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("ssh-keygen failed: %#v", err)
		}

		return f(privateKey, publicKey)
	})
}
