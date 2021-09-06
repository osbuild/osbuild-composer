package rpmmd

import (
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type RPM struct {
	Type      string  `json:"type"` // must be 'rpm'
	Name      string  `json:"name"`
	Version   string  `json:"version"`
	Release   string  `json:"release"`
	Epoch     *string `json:"epoch,omitempty"`
	Arch      string  `json:"arch"`
	Sigmd5    string  `json:"sigmd5"`
	Signature *string `json:"signature"`
}

func OSBuildMetadataToRPMs(stagesMetadata map[string]osbuild.StageMetadata) []RPM {
	rpms := make([]RPM, 0)
	for _, md := range stagesMetadata {
		switch metadata := md.(type) {
		case *osbuild.RPMStageMetadata:
			for _, pkg := range metadata.Packages {
				rpms = append(rpms, RPM{
					Type:      "rpm",
					Name:      pkg.Name,
					Epoch:     pkg.Epoch,
					Version:   pkg.Version,
					Release:   pkg.Release,
					Arch:      pkg.Arch,
					Sigmd5:    pkg.SigMD5,
					Signature: PackageMetadataToSignature(pkg),
				})
			}
		default:
			continue
		}
	}
	return rpms
}

func PackageMetadataToSignature(pkg osbuild.RPMPackageMetadata) *string {
	if pkg.SigGPG != "" {
		return &pkg.SigGPG
	} else if pkg.SigPGP != "" {
		return &pkg.SigPGP
	}
	return nil
}
