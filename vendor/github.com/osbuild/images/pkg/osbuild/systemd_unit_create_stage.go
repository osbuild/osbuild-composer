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

const (
	unitFilenameRegex = "^[\\w:.\\\\-]+[@]{0,1}[\\w:.\\\\-]*\\.(service|mount|socket|swap)$"

	// This is less strict than the corresponding regex in osbuild. In osbuild,
	// we use lookaheads to validate paths, whereas in images we use an invalid
	// path regex and invert the check. Validating the paths for the three
	// values that take a path parameter (file, append, and truncate), would
	// make the regex too complicated, so we validate a bit less strictly and
	// let osbuild handle the final validation.
	systemdStandardOutputRegex = "^(inherit|null|tty|journal|kmsg|journal\\+console|kmsg\\+console|file:.+|append:.+|truncate:.+|socket|fd:.+)$"
)

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
	Description              string   `json:"Description,omitempty" yaml:"Description,omitempty"`
	DefaultDependencies      *bool    `json:"DefaultDependencies,omitempty" yaml:"DefaultDependencies,omitempty"`
	ConditionPathExists      []string `json:"ConditionPathExists,omitempty" yaml:"ConditionPathExists,omitempty"`
	ConditionPathIsDirectory []string `json:"ConditionPathIsDirectory,omitempty" yaml:"ConditionPathIsDirectory,omitempty"`
	Requires                 []string `json:"Requires,omitempty" yaml:"Requires,omitempty"`
	Wants                    []string `json:"Wants,omitempty" yaml:"Wants,omitempty"`
	After                    []string `json:"After,omitempty" yaml:"After,omitempty"`
	Before                   []string `json:"Before,omitempty" yaml:"Before,omitempty"`
}

type ServiceSection struct {
	Type            SystemdServiceType    `json:"Type,omitempty" yaml:"Type,omitempty"`
	RemainAfterExit bool                  `json:"RemainAfterExit,omitempty" yaml:"RemainAfterExit,omitempty"`
	ExecStartPre    []string              `json:"ExecStartPre,omitempty" yaml:"ExecStartPre,omitempty"`
	ExecStopPost    []string              `json:"ExecStopPost,omitempty" yaml:"ExecStopPost,omitempty"`
	ExecStart       []string              `json:"ExecStart,omitempty" yaml:"ExecStart,omitempty"`
	Environment     []EnvironmentVariable `json:"Environment,omitempty" yaml:"Environment,omitempty"`
	EnvironmentFile []string              `json:"EnvironmentFile,omitempty" yaml:"EnvironmentFile,omitempty"`
	StandardOutput  string                `json:"StandardOutput,omitempty" yaml:"StandardOutput,omitempty"`
}

type MountSection struct {
	What    string `json:"What" yaml:"What"`
	Where   string `json:"Where" yaml:"Where"`
	Type    string `json:"Type,omitempty" yaml:"Type,omitempty"`
	Options string `json:"Options,omitempty" yaml:"Options,omitempty"`
}

type SwapSection struct {
	What       string `json:"What" yaml:"What"`
	Priority   *int   `json:"Priority,omitempty" yaml:"Priority,omitempty"`
	Options    string `json:"Options,omitempty" yaml:"Options,omitempty"`
	TimeoutSec string `json:"TimeoutSec,omitempty" yaml:"TimeoutSec,omitempty"`
}

type SocketSection struct {
	Service                string `json:"Service,omitempty" yaml:"Service,omitempty"`
	ListenStream           string `json:"ListenStream,omitempty" yaml:"ListenStream,omitempty"`
	ListenDatagram         string `json:"ListenDatagram,omitempty" yaml:"ListenDatagram,omitempty"`
	ListenSequentialPacket string `json:"ListenSequentialPacket,omitempty" yaml:"ListenSequentialPacket,omitempty"`
	ListenFifo             string `json:"ListenFifo,omitempty" yaml:"ListenFifo,omitempty"`
	SocketUser             string `json:"SocketUser,omitempty" yaml:"SocketUser,omitempty"`
	SocketGroup            string `json:"SocketGroup,omitempty" yaml:"SocketGroup,omitempty"`
	SocketMode             string `json:"SocketMode,omitempty" yaml:"SocketMode,omitempty"`
	DirectoryMode          string `json:"DirectoryMode,omitempty" yaml:"DirectoryMode,omitempty"`
	Accept                 string `json:"Accept,omitempty" yaml:"Accept,omitempty"`
	RuntimeDirectory       string `json:"RuntimeDirectory,omitempty" yaml:"RuntimeDirectory,omitempty"`
	RemoveOnStop           string `json:"RemoveOnStop,omitempty" yaml:"RemoveOnStop,omitempty"`
}

type InstallSection struct {
	RequiredBy []string `json:"RequiredBy,omitempty" yaml:"RequiredBy,omitempty"`
	WantedBy   []string `json:"WantedBy,omitempty" yaml:"WantedBy,omitempty"`
}

type SystemdUnit struct {
	Unit    *UnitSection    `json:"Unit" yaml:"Unit"`
	Service *ServiceSection `json:"Service,omitempty" yaml:"Service,omitempty"`
	Mount   *MountSection   `json:"Mount,omitempty" yaml:"Mount,omitempty"`
	Socket  *SocketSection  `json:"Socket,omitempty" yaml:"Socket,omitempty"`
	Swap    *SwapSection    `json:"Swap,omitempty" yaml:"Swap,omitempty"`
	Install *InstallSection `json:"Install,omitempty" yaml:"Install,omitempty"`
}

type SystemdUnitCreateStageOptions struct {
	Filename string          `json:"filename" yaml:"filename"`
	UnitType unitType        `json:"unit-type,omitempty" yaml:"unit-type,omitempty"` // unitType defined in ./systemd_unit_stage.go
	UnitPath SystemdUnitPath `json:"unit-path,omitempty" yaml:"unit-path,omitempty"`
	Config   SystemdUnit     `json:"config" yaml:"config"`
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
				return fmt.Errorf("variable name %q does not conform to schema (%s)", envVar.Key, envVarRegex)
			}
		}

		if stdout := service.StandardOutput; stdout != "" && !regexp.MustCompile(systemdStandardOutputRegex).MatchString(stdout) {
			return fmt.Errorf("StandardOutput value %q does not conform to schema (%s)", service.StandardOutput, systemdStandardOutputRegex)
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
