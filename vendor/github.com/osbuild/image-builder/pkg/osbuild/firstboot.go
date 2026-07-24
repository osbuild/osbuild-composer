package osbuild

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/customizations/firstboot"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
)

const (
	firstbootMarkerPath = "/var/local/osbuild-first-boot/.run"
	firstbootScriptDir  = "/usr/libexec/osbuild-first-boot"
)

var firstbootBaseAfter = []string{"network-online.target", "osbuild-first-boot.service"}

// GenFirstbootFromOptions processes the firstboot options and returns a list of CA certificates to
// include in the image, a list of directory nodes, a list of file nodes to create the firstboot scripts, and
// systemd units to run the scripts on first boot.
func GenFirstbootFromOptions(fbo *firstboot.FirstbootOptions) ([]string, []*fsnode.Directory, []*fsnode.File, []*SystemdUnitCreateStageOptions, error) {
	if fbo == nil {
		return nil, nil, nil, nil, nil
	}

	var certs []string
	var dirs []*fsnode.Directory
	var files []*fsnode.File
	var units []*SystemdUnitCreateStageOptions

	if len(fbo.Scripts) > 0 {
		d, err := fsnode.NewDirectory(firstbootScriptDir, common.ToPtr(fs.FileMode(0755)), "root", "root", true)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error creating firstboot script directory node: %w", err)
		}
		dirs = append(dirs, d)

		d, err = fsnode.NewDirectory("/var/local/osbuild-first-boot", common.ToPtr(fs.FileMode(0755)), "root", "root", true)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error creating firstboot marker directory node: %w", err)
		}
		dirs = append(dirs, d)

		f, err := fsnode.NewFile(firstbootMarkerPath, common.ToPtr(fs.FileMode(0770)), "root", "root", []byte{})
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error creating firstboot marker node: %w", err)
		}
		files = append(files, f)
	}

	var prevUnit string
	for i, script := range fbo.Scripts {
		unitFilename := script.Filename + ".service"

		exec := fmt.Sprintf("%s/%s", firstbootScriptDir, script.Filename)
		f, err := fsnode.NewFile(exec, common.ToPtr(fs.FileMode(0770)), "root", "root", []byte(script.Contents))
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error creating firstboot file node %q: %w", exec, err)
		}
		files = append(files, f)

		execStart := exec
		if script.IgnoreFailure {
			execStart = "-" + exec
		}

		after := append([]string{}, firstbootBaseAfter...)
		after = append(after, script.After...)
		if prevUnit != "" {
			after = append(after, prevUnit)
		}
		after = dedupeOrdered(after)

		var execStartPre []string
		if i == len(fbo.Scripts)-1 {
			execStartPre = []string{"/usr/bin/rm " + firstbootMarkerPath}
		}

		unit := SystemdUnit{
			Unit: &UnitSection{
				ConditionPathExists: []string{firstbootMarkerPath},
				Wants:               []string{"network-online.target"},
				After:               after,
				Before:              slices.Clone(script.Before),
			},
			Service: &ServiceSection{
				Type:            OneshotServiceType,
				ExecStart:       []string{execStart},
				ExecStartPre:    execStartPre,
				RemainAfterExit: true,
			},
			Install: &InstallSection{
				WantedBy: []string{"basic.target"},
			},
		}

		units = append(units, &SystemdUnitCreateStageOptions{
			Filename: unitFilename,
			Config:   unit,
			UnitType: SystemUnitType,
			UnitPath: UsrUnitPath,
		})
		prevUnit = unitFilename

		certs = append(certs, script.Certs...)
	}

	return certs, dirs, files, units, nil
}

func dedupeOrdered(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
