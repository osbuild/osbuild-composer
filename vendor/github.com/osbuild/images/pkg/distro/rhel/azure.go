package rhel

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/osbuild"
)

// Dataloss warning script for Azure images.
//
//go:embed temp-disk-dataloss-warning.sh
var azureDatalossWarningScriptContent string

// Returns a filenode that embeds a script and a systemd unit to run it on
// every boot.
// The script writes a file named DATALOSS_WARNING_README.txt to the root of an
// Azure ephemeral resource disk, if one is mounted, as a warning against using
// the disk for data storage.
// https://docs.microsoft.com/en-us/azure/virtual-machines/linux/managed-disks-overview#temporary-disk
func CreateAzureDatalossWarningScriptAndUnit() (*fsnode.File, *osbuild.SystemdUnitCreateStageOptions, error) {
	datalossWarningScriptPath := "/usr/local/sbin/temp-disk-dataloss-warning"
	datalossWarningScript, err := fsnode.NewFile(datalossWarningScriptPath, common.ToPtr(os.FileMode(0755)), nil, nil, []byte(azureDatalossWarningScriptContent))
	if err != nil {
		return nil, nil, fmt.Errorf("rhel/azure: error creating file node for dataloss warning script: %w", err)
	}

	systemdUnit := &osbuild.SystemdUnitCreateStageOptions{
		Filename: "temp-disk-dataloss-warning.service",
		UnitType: osbuild.SystemUnitType,
		UnitPath: osbuild.EtcUnitPath,
		Config: osbuild.SystemdUnit{
			Unit: &osbuild.UnitSection{
				Description: "Azure temporary resource disk dataloss warning file creation",
				After:       []string{"multi-user.target", "cloud-final.service"},
			},
			Service: &osbuild.ServiceSection{
				Type:           osbuild.OneshotServiceType,
				ExecStart:      []string{datalossWarningScriptPath},
				StandardOutput: "journal+console",
			},
			Install: &osbuild.InstallSection{
				WantedBy: []string{"default.target"},
			},
		},
	}

	return datalossWarningScript, systemdUnit, nil
}
