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

func newCertificateKeyPair(CA, CAkey, commonName string, subjectAlternativeNames []string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}

	args := []string{
		"-out", ckp.certificate(),
		"-keyout", ckp.key(),
		"-CA", CA,
		"-CAkey", CAkey,
		"-cn", commonName,
		"-san",
	}
	args = append(args, subjectAlternativeNames...)

	cmd := exec.Command(
		"/usr/libexec/osbuild-composer-test/x509/generate-certificate",
		args...,
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot create the certificate: %v", err)
	}

	return &ckp, nil
}

func newSelfSignedCertificateKeyPair(commonName string, subjectAlternativeNames []string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}

	args := []string{
		"-selfsigned",
		"-out", ckp.certificate(),
		"-keyout", ckp.key(),
		"-cn", commonName,
		"-san",
	}
	args = append(args, subjectAlternativeNames...)

	cmd := exec.Command(
		"/usr/libexec/osbuild-composer-test/x509/generate-certificate",
		args...,
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a self-signed certificate: %v", err)
	}

	return &ckp, nil
}
