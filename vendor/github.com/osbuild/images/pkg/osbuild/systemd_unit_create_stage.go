package osbuild

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

const unitFilenameRegex = "^[\\w:.\\\\-]+[@]{0,1}[\\w:.\\\\-]*\\.(service|mount|socket|swap)$"

type SystemdServiceType string
type SystemdUnitPath string

const (
	SimpleServiceType       SystemdServiceType = "simple"
	ExecServiceType         SystemdServiceType = "exec"
	ForkingServiceType      SystemdServiceType = "forking"
	OneshotServiceType      SystemdServiceType = "oneshot"
	DbusServiceType         SystemdServiceType = "dbus"
	NotifyServiceType       SystemdServiceType = "notify"
	NotifyReloadServiceType SystemdServiceType = "notify-reload"
	IdleServiceType         SystemdServiceType = "idle"

	EtcUnitPath SystemdUnitPath = "etc"
	UsrUnitPath SystemdUnitPath = "usr"
)

type UnitSection struct {
	Description              string   `json:"Description,omitempty"`
	DefaultDependencies      *bool    `json:"DefaultDependencies,omitempty"`
	ConditionPathExists      []string `json:"ConditionPathExists,omitempty"`
	ConditionPathIsDirectory []string `json:"ConditionPathIsDirectory,omitempty"`
	Requires                 []string `json:"Requires,omitempty"`
	Wants                    []string `json:"Wants,omitempty"`
	After                    []string `json:"After,omitempty"`
	Before                   []string `json:"Before,omitempty"`
}

type ServiceSection struct {
	Type            SystemdServiceType    `json:"Type,omitempty"`
	RemainAfterExit bool                  `json:"RemainAfterExit,omitempty"`
	ExecStartPre    []string              `json:"ExecStartPre,omitempty"`
	ExecStopPost    []string              `json:"ExecStopPost,omitempty"`
	ExecStart       []string              `json:"ExecStart,omitempty"`
	Environment     []EnvironmentVariable `json:"Environment,omitempty"`
	EnvironmentFile []string              `json:"EnvironmentFile,omitempty"`
}

type MountSection struct {
	What    string `json:"What"`
	Where   string `json:"Where"`
	Type    string `json:"Type,omitempty"`
	Options string `json:"Options,omitempty"`
}

type SwapSection struct {
	What       string `json:"What"`
	Priority   *int   `json:"Priority,omitempty"`
	Options    string `json:"Options,omitempty"`
	TimeoutSec string `json:"TimeoutSec,omitempty"`
}

type SocketSection struct {
	Service                string `json:"Service,omitempty"`
	ListenStream           string `json:"ListenStream,omitempty"`
	ListenDatagram         string `json:"ListenDatagram,omitempty"`
	ListenSequentialPacket string `json:"ListenSequentialPacket,omitempty"`
	ListenFifo             string `json:"ListenFifo,omitempty"`
	SocketUser             string `json:"SocketUser,omitempty"`
	SocketGroup            string `json:"SocketGroup,omitempty"`
	SocketMode             string `json:"SocketMode,omitempty"`
	DirectoryMode          string `json:"DirectoryMode,omitempty"`
	Accept                 string `json:"Accept,omitempty"`
	RuntimeDirectory       string `json:"RuntimeDirectory,omitempty"`
	RemoveOnStop           string `json:"RemoveOnStop,omitempty"`
}

type InstallSection struct {
	RequiredBy []string `json:"RequiredBy,omitempty"`
	WantedBy   []string `json:"WantedBy,omitempty"`
}

type SystemdUnit struct {
	Unit    *UnitSection    `json:"Unit"`
	Service *ServiceSection `json:"Service,omitempty"`
	Mount   *MountSection   `json:"Mount,omitempty"`
	Socket  *SocketSection  `json:"Socket,omitempty"`
	Swap    *SwapSection    `json:"Swap,omitempty"`
	Install *InstallSection `json:"Install,omitempty"`
}

type SystemdUnitCreateStageOptions struct {
	Filename string          `json:"filename"`
	UnitType unitType        `json:"unit-type,omitempty"` // unitType defined in ./systemd_unit_stage.go
	UnitPath SystemdUnitPath `json:"unit-path,omitempty"`
	Config   SystemdUnit     `json:"config"`
}

func (SystemdUnitCreateStageOptions) isStageOptions() {}

func (o *SystemdUnitCreateStageOptions) validateService() error {
	if o.Config.Service == nil {
		return fmt.Errorf("systemd service unit %q requires a Service section", o.Filename)
	}
	if o.Config.Install == nil {
		return fmt.Errorf("systemd service unit %q requires an Install section", o.Filename)
	}

	if o.Config.Mount != nil {
		return fmt.Errorf("systemd service unit %q contains invalid section Mount", o.Filename)
	}
	if o.Config.Socket != nil {
		return fmt.Errorf("systemd service unit %q contains invalid section Socket", o.Filename)
	}
	if o.Config.Swap != nil {
		return fmt.Errorf("systemd service unit %q contains invalid section Swap", o.Filename)
	}

	vre := regexp.MustCompile(envVarRegex)
	if service := o.Config.Service; service != nil {
		for _, envVar := range service.Environment {
			if !vre.MatchString(envVar.Key) {
				return fmt.Errorf("variable name %q doesn't conform to schema (%s)", envVar.Key, envVarRegex)
			}
		}
	}

	return nil
}

func (o *SystemdUnitCreateStageOptions) validateMount() error {
	if o.Config.Mount == nil {
		return fmt.Errorf("systemd mount unit %q requires a Mount section", o.Filename)
	}

	if o.Config.Swap != nil {
		return fmt.Errorf("systemd mount unit %q contains invalid section Swap", o.Filename)
	}
	if o.Config.Service != nil {
		return fmt.Errorf("systemd mount unit %q contains invalid section Service", o.Filename)
	}
	if o.Config.Socket != nil {
		return fmt.Errorf("systemd mount unit %q contains invalid section Socket", o.Filename)
	}
	if o.Config.Swap != nil {
		return fmt.Errorf("systemd mount unit %q contains invalid section Swap", o.Filename)
	}

	if o.Config.Mount.What == "" {
		return fmt.Errorf("What option for Mount section of systemd unit %q is required", o.Filename)
	}

	if o.Config.Mount.Where == "" {
		return fmt.Errorf("Where option for Mount section of systemd unit %q is required", o.Filename)
	}

	return nil
}

func (o *SystemdUnitCreateStageOptions) validateSwap() error {
	if o.Config.Swap == nil {
		return fmt.Errorf("systemd swap unit %q requires a Swap section", o.Filename)
	}

	if o.Config.Mount != nil {
		return fmt.Errorf("systemd swap unit %q contains invalid section Mount", o.Filename)
	}
	if o.Config.Service != nil {
		return fmt.Errorf("systemd swap unit %q contains invalid section Service", o.Filename)
	}
	if o.Config.Socket != nil {
		return fmt.Errorf("systemd swap unit %q contains invalid section Socket", o.Filename)
	}

	return nil
}

func (o *SystemdUnitCreateStageOptions) validateSocket() error {
	if o.Config.Socket == nil {
		return fmt.Errorf("systemd socket unit %q requires a Socket section", o.Filename)
	}

	if o.Config.Mount != nil {
		return fmt.Errorf("systemd socket unit %q contains invalid section Mount", o.Filename)
	}
	if o.Config.Service != nil {
		return fmt.Errorf("systemd socket unit %q contains invalid section Service", o.Filename)
	}
	if o.Config.Swap != nil {
		return fmt.Errorf("systemd socket unit %q contains invalid section Swap", o.Filename)
	}

	return nil
}

func (o *SystemdUnitCreateStageOptions) validate() error {
	fre := regexp.MustCompile(unitFilenameRegex)
	if !fre.MatchString(o.Filename) {
		return fmt.Errorf("invalid filename %q for systemd unit: does not conform to schema (%s)", o.Filename, unitFilenameRegex)
	}

	switch filepath.Ext(o.Filename) {
	case ".service":
		return o.validateService()
	case ".mount":
		return o.validateMount()
	case ".swap":
		return o.validateSwap()
	case ".socket":
		return o.validateSocket()
	default:
		// this should be caught by the regex
		return fmt.Errorf("invalid filename %q for systemd unit: extension must be one of .service, .mount, .swap, or .socket", o.Filename)
	}
}

func NewSystemdUnitCreateStage(options *SystemdUnitCreateStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd.unit.create",
		Options: options,
	}
}

// GenSystemdMountStages generates a collection of
// org.osbuild.systemd.unit.create stages with options to create systemd mount
// units, one for each mountpoint in the partition table.
func GenSystemdMountStages(pt *disk.PartitionTable) ([]*Stage, error) {
	mountStages := make([]*Stage, 0)
	unitNames := make([]string, 0)

	genOption := func(ent disk.FSTabEntity, path []disk.Entity) error {
		fsSpec := ent.GetFSSpec()
		fsOptions, err := ent.GetFSTabOptions()
		if err != nil {
			return err
		}

		options := &SystemdUnitCreateStageOptions{
			UnitPath: EtcUnitPath, // create all mount units in /etc/systemd/
			Config: SystemdUnit{
				Unit: &UnitSection{
					// Adds the following dependencies for mount units (systemd.mount(5)):
					//  - Before=umount.target
					//  - Conflicts=umount.target
					//  - After=local-fs-pre.target
					//  - Before=local-fs.target
					// and the following for swap units (systemd.swap(5)):
					//  - Before=umount.target
					//  - Conflicts=umount.target
					DefaultDependencies: common.ToPtr(true),
				},
				Install: &InstallSection{
					WantedBy: []string{"multi-user.target"},
				},
			},
		}

		device := filepath.Join("/dev/disk/by-uuid", strings.ToLower(fsSpec.UUID))
		if isFATVolID(fsSpec.UUID) {
			// vfat IDs aren't lowercased
			device = filepath.Join("/dev/disk/by-uuid", fsSpec.UUID)
		}

		switch ent.GetFSType() {
		case "swap":
			options.Filename = fmt.Sprintf("%s.swap", pathEscape(device))
			options.Config.Swap = &SwapSection{
				What:    device,
				Options: fsOptions.MntOps,
			}
		default:
			options.Filename = fmt.Sprintf("%s.mount", pathEscape(ent.GetFSFile()))
			options.Config.Mount = &MountSection{
				What:    device,
				Where:   ent.GetFSFile(),
				Type:    ent.GetFSType(),
				Options: fsOptions.MntOps,
			}
		}

		mountStages = append(mountStages, NewSystemdUnitCreateStage(options))
		unitNames = append(unitNames, options.Filename)
		return nil
	}

	err := pt.ForEachFSTabEntity(genOption)
	if err != nil {
		return nil, err
	}

	// sort the entries by filename for stable ordering
	slices.SortFunc(mountStages, func(a, b *Stage) int {
		optsa := a.Options.(*SystemdUnitCreateStageOptions)
		optsb := b.Options.(*SystemdUnitCreateStageOptions)

		// this sorter is not guaranteed to be stable, but the unit Filenames
		// are unique
		switch {
		case optsa.Filename < optsb.Filename:
			return -1
		case optsa.Filename > optsb.Filename:
			return 1
		}
		panic(fmt.Sprintf("error sorting systemd unit mount stages: possible duplicate mount unit filenames: %q %q", optsa.Filename, optsb.Filename))
	})

	// sort the unit names for the systemd (enable) stage for stable ordering
	slices.Sort(unitNames)

	if len(unitNames) > 0 {
		enableStage := NewSystemdStage(&SystemdStageOptions{
			EnabledServices: unitNames,
		})
		mountStages = append(mountStages, enableStage)
	}

	return mountStages, nil
}
