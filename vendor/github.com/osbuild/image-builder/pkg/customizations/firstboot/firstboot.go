package firstboot

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/pkg/shutil"
)

const firstbootPrefix = "osbuild-first-boot-"

type FirstbootOptions struct {
	Scripts []Script
}

type Script struct {
	Filename      string
	Contents      string
	IgnoreFailure bool
	Certs         []string
	After         []string
	Before        []string
}

var ErrFirstbootAlreadySet = errors.New("firstboot customization already set")
var ErrFirstbootNameTaken = errors.New("firstboot name already taken")

func validateCustomName(name string) error {
	if name != filepath.Base(name) {
		return fmt.Errorf("firstboot name %q is invalid: must be a simple filename without path separators", name)
	}
	if name == "satellite" || name == "aap" || strings.HasPrefix(name, "custom") {
		return fmt.Errorf("firstboot name %q is reserved", name)
	}
	return nil
}

func scriptFromCommon(common blueprint.FirstbootCommonCustomization, filename, contents string, certs []string) Script {
	return Script{
		Filename:      filename,
		Contents:      contents,
		IgnoreFailure: common.IgnoreFailure,
		Certs:         certs,
		After:         slices.Clone(common.After),
		Before:        slices.Clone(common.Before),
	}
}

// FirstbootOptionsFromBP converts blueprint firstboot customizations into the
// internal representation used to generate osbuild stages.
//
// Script basenames are assigned as follows:
//   - satellite scripts use osbuild-first-boot-satellite
//   - aap scripts use osbuild-first-boot-aap
//   - named custom scripts use osbuild-first-boot-{name}
//   - unnamed custom scripts use osbuild-first-boot-custom-N
//
// At most one satellite and one aap script are allowed. Custom names must be a
// local path segment, must be unique, and must not be "satellite", "aap", or
// start with "custom".
//
// Returns ErrFirstbootAlreadySet when a second satellite or aap script is
// present, ErrFirstbootNameTaken when a custom name is reused, or an error when
// a custom name is invalid or reserved.
func FirstbootOptionsFromBP(bpFirstboot blueprint.FirstbootCustomization) (*FirstbootOptions, error) {
	fo := &FirstbootOptions{}
	used := make(map[string]struct{})
	var customN int
	var satSet, aapSet bool

	for _, fbsc := range bpFirstboot.Scripts {
		cust, sat, aap, err := fbsc.SelectUnion()
		if err != nil {
			return nil, err
		}

		if cust != nil {
			var filename string
			if cust.Name == "" {
				for {
					customN++
					filename = fmt.Sprintf("%scustom-%d", firstbootPrefix, customN)
					if _, ok := used[filename]; !ok {
						break
					}
				}
			} else {
				if err := validateCustomName(cust.Name); err != nil {
					return nil, err
				}
				filename = firstbootPrefix + cust.Name
				if _, ok := used[filename]; ok {
					return nil, fmt.Errorf("%w: %s", ErrFirstbootNameTaken, cust.Name)
				}
			}
			used[filename] = struct{}{}
			fo.Scripts = append(fo.Scripts, scriptFromCommon(
				cust.FirstbootCommonCustomization,
				filename,
				cust.Contents,
				nil,
			))
		}

		if sat != nil {
			if satSet {
				return nil, fmt.Errorf("%w: satellite", ErrFirstbootAlreadySet)
			}
			satSet = true
			fo.Scripts = append(fo.Scripts, scriptFromCommon(
				sat.FirstbootCommonCustomization,
				firstbootPrefix+"satellite",
				sat.Command,
				sat.CACerts,
			))
		}

		if aap != nil {
			if aapSet {
				return nil, fmt.Errorf("%w: aap", ErrFirstbootAlreadySet)
			}
			aapSet = true

			contents := fmt.Sprintf("#!/usr/bin/bash\ncurl -i --data %s %s\n",
				shutil.Quote("host_config_key="+aap.HostConfigKey),
				shutil.Quote(aap.JobTemplateURL),
			)
			fo.Scripts = append(fo.Scripts, scriptFromCommon(
				aap.FirstbootCommonCustomization,
				firstbootPrefix+"aap",
				contents,
				aap.CACerts,
			))
		}
	}

	return fo, nil
}
