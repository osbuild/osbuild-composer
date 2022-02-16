package osbuild2

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type RPMStageOptions struct {
	// Array of GPG key contents to import
	GPGKeys []string `json:"gpgkeys,omitempty"`

	// Prevent dracut from running
	DisableDracut bool `json:"disable_dracut,omitempty"`

	Exclude *Exclude `json:"exclude,omitempty"`
}

type Exclude struct {
	// Do not install documentation.
	Docs bool `json:"docs,omitempty"`
}

// RPMPackage represents one RPM, as referenced by its content hash
// (checksum). The files source must indicate where to fetch the given
// RPM. If CheckGPG is `true` the RPM must be signed with one of the
// GPGKeys given in the RPMStageOptions.
type RPMPackage struct {
	Checksum string `json:"checksum"`
	CheckGPG bool   `json:"check_gpg,omitempty"`
}

func (RPMStageOptions) isStageOptions() {}

type RPMStageInputs struct {
	Packages *RPMStageInput `json:"packages"`
}

func (RPMStageInputs) isStageInputs() {}

type RPMStageInput struct {
	inputCommon
	References RPMStageReferences `json:"references"`
}

func (RPMStageInput) isStageInput() {}

type RPMStageReferences []string

func (RPMStageReferences) isReferences() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions, inputs *RPMStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm",
		Inputs:  inputs,
		Options: options,
	}
}

// RPMStageMetadata gives the set of packages installed by the RPM stage
type RPMStageMetadata struct {
	Packages []RPMPackageMetadata `json:"packages"`
}

// RPMPackageMetadata contains the metadata extracted from one RPM header
type RPMPackageMetadata struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Release string  `json:"release"`
	Epoch   *string `json:"epoch"`
	Arch    string  `json:"arch"`
	SigMD5  string  `json:"sigmd5"`
	SigPGP  string  `json:"sigpgp"`
	SigGPG  string  `json:"siggpg"`
}

func (RPMStageMetadata) isStageMetadata() {}

func OSBuildMetadataToRPMs(stagesMetadata map[string]StageMetadata) []rpmmd.RPM {
	rpms := make([]rpmmd.RPM, 0)
	for _, md := range stagesMetadata {
		switch metadata := md.(type) {
		case *RPMStageMetadata:
			for _, pkg := range metadata.Packages {
				rpms = append(rpms, rpmmd.RPM{
					Type:      "rpm",
					Name:      pkg.Name,
					Epoch:     pkg.Epoch,
					Version:   pkg.Version,
					Release:   pkg.Release,
					Arch:      pkg.Arch,
					Sigmd5:    pkg.SigMD5,
					Signature: RPMPackageMetadataToSignature(pkg),
				})
			}
		default:
			continue
		}
	}
	return rpms
}

func RPMPackageMetadataToSignature(pkg RPMPackageMetadata) *string {
	if pkg.SigGPG != "" {
		return &pkg.SigGPG
	} else if pkg.SigPGP != "" {
		return &pkg.SigPGP
	}
	return nil
}

func NewRpmStageSourceFilesInputs(specs []rpmmd.PackageSpec) *RPMStageInputs {
	stageInput := new(RPMStageInput)
	stageInput.Type = "org.osbuild.files"
	stageInput.Origin = "org.osbuild.source"
	stageInput.References = pkgRefs(specs)
	return &RPMStageInputs{Packages: stageInput}
}

func pkgRefs(specs []rpmmd.PackageSpec) RPMStageReferences {
	refs := make([]string, len(specs))
	for idx, pkg := range specs {
		refs[idx] = pkg.Checksum
	}
	return refs
}
