package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
)

type certificateKeyPair struct {
	baseDir string
}

func (ckp certificateKeyPair) remove() {
	err := os.RemoveAll(ckp.baseDir)
	if err != nil {
		log.Printf("cannot delete the certificate key pair: %v", err)
	}
}

func (ckp certificateKeyPair) certificate() string {
	return path.Join(ckp.baseDir, "crt")
}

func (ckp certificateKeyPair) key() string {
	return path.Join(ckp.baseDir, "key")
}

func newCertificateKeyPair(CA, CAkey, commonName string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}
	certificateRequest := path.Join(dir, "csr")

	cmd := exec.Command(
		"openssl", "req", "-new", "-nodes",
		"-subj", fmt.Sprintf("/CN=%s", commonName),
		"-keyout", ckp.key(),
		"-out", certificateRequest,
	)

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a private key and a certificate request: %v", err)
	}

	defer os.Remove(certificateRequest)

	cmd = exec.Command(
		"openssl", "x509", "-req", "-CAcreateserial",
		"-in", certificateRequest,
		"-CA", CA,
		"-CAkey", CAkey,
		"-out", ckp.certificate(),
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot sign the certificate: %v", err)
	}

	return &ckp, nil
}

func newSelfSignedCertificateKeyPair(commonName string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}

	cmd := exec.Command(
		"openssl", "req", "-nodes", "-x509",
		"-subj", fmt.Sprintf("/CN=%s", commonName),
		"-out", ckp.certificate(),
		"-keyout", ckp.key(),
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a self-signed certificate: %v", err)
	}

	return &ckp, nil
}
