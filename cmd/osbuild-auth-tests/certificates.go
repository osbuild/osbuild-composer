//go:build integration
// +build integration

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

const (
	opensslConfig    = "/usr/share/tests/osbuild-composer/x509/openssl.cnf"
	osbuildCAExt     = "osbuild_ca_ext"
	osbuildClientExt = "osbuild_client_ext"
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

func newSelfSignedCertificateKeyPair(subj string) (*certificateKeyPair, error) {
	dir, err := os.MkdirTemp("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}

	//nolint:gosec
	cmd := exec.Command(
		"openssl", "req", "-nodes", "-x509",
		"-subj", subj,
		"-out", ckp.certificate(),
		"-keyout", ckp.key(),
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a self-signed certificate: %v", err)
	}

	return &ckp, nil
}

type ca struct {
	BaseDir string
}

func (c ca) remove() {
	err := os.RemoveAll(c.BaseDir)
	if err != nil {
		log.Printf("cannot delete the ca: %v", err)
	}
}

func (c ca) certificate() string {
	return path.Join(c.BaseDir, "ca.cert.pem")
}

func (c ca) key() string {
	return path.Join(c.BaseDir, "private", "ca.key.pem")
}

func newCA(subj string) (*ca, error) {
	baseDir, err := os.MkdirTemp("", "osbuild-auth-tests-ca")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary dir for a new CA: %v", err)
	}

	err = os.Mkdir(path.Join(baseDir, "certs"), 0700)
	if err != nil {
		innerErr := os.RemoveAll(baseDir)
		if innerErr != nil {
			log.Print(innerErr)
		}
		return nil, fmt.Errorf("cannot create certs dir for the new CA: %v", err)
	}

	err = os.Mkdir(path.Join(baseDir, "private"), 0700)
	if err != nil {
		innerErr := os.RemoveAll(baseDir)
		if innerErr != nil {
			log.Print(innerErr)
		}
		return nil, fmt.Errorf("cannot create private dir for the new CA: %v", err)
	}

	f, err := os.Create(path.Join(baseDir, "index.txt"))
	if err != nil {
		innerErr := os.RemoveAll(baseDir)
		if innerErr != nil {
			log.Print(innerErr)
		}
		return nil, fmt.Errorf("cannot create index file for the new CA: %v", err)
	}
	f.Close()

	c := ca{
		BaseDir: baseDir,
	}

	//nolint:gosec
	cmd := exec.Command(
		"openssl", "req",
		"-config", opensslConfig,
		"-new", "-nodes", "-x509", "-extensions", osbuildCAExt,
		"-subj", subj,
		"-keyout", c.key(),
		"-out", c.certificate(),
	)

	err = cmd.Run()
	if err != nil {
		innerErr := os.RemoveAll(baseDir)
		if innerErr != nil {
			log.Print(innerErr)
		}
		return nil, fmt.Errorf("cannot create the CA: %v", err)
	}

	return &c, nil
}

func (c ca) newCertificateKeyPair(subj, extensions, addext string) (*certificateKeyPair, error) {
	dir, err := os.MkdirTemp("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}
	certificateRequest := path.Join(dir, "csr")

	args := []string{
		"req", "-new", "-nodes",
		"-subj", subj,
		"-keyout", ckp.key(),
		"-out", certificateRequest,
		"-config", opensslConfig,
	}

	if addext != "" {
		args = append(args, "-addext", addext)
	}

	cmd := exec.Command(
		"openssl",
		args...,
	)

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a private key and a certificate request: %v", err)
	}

	defer os.Remove(certificateRequest)

	cmd = exec.Command(
		"openssl", "ca",
		"-batch",
		"-config", opensslConfig,
		"-extensions", extensions,
		"-in", certificateRequest,
		"-out", ckp.certificate(),
	)
	// this command must be run in the CA base directory
	cmd.Dir = c.BaseDir

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot sign the certificate: %v", err)
	}

	return &ckp, nil
}
