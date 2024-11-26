package osbuild

import (
	"encoding/pem"
	"fmt"
	"path/filepath"

	"github.com/osbuild/images/pkg/cert"
	"github.com/osbuild/images/pkg/customizations/fsnode"
)

func NewCAStageStage() *Stage {
	return &Stage{
		Type: "org.osbuild.pki.update-ca-trust",
	}
}

func NewCAFileNodes(bundle string) ([]*fsnode.File, error) {
	var files []*fsnode.File
	certs, err := cert.ParseCerts(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificates: %v", err)
	}

	for _, c := range certs {
		path := filepath.Join("/etc/pki/ca-trust/source/anchors", filepath.Base(c.SerialNumber.Text(16))+".pem")
		f, err := fsnode.NewFile(path, nil, "root", "root", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}))
		if err != nil {
			panic(err)
		}
		files = append(files, f)
	}

	return files, nil
}
