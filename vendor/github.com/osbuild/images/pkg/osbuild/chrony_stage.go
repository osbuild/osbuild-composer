package osbuild

import (
	"fmt"
	"regexp"
)

const (
	chronyConfStageType  = "org.osbuild.chrony"
	chronyStagePermRegex = `^[0-7]{4}$`
)

type ChronyStageOptions struct {
	Servers   []ChronyConfigServer   `json:"servers,omitempty"`
	Refclocks []ChronyConfigRefclock `json:"refclocks,omitempty"`
	LeapsecTz *string                `json:"leapsectz,omitempty"`
}

func (ChronyStageOptions) isStageOptions() {}

func (o ChronyStageOptions) validate() error {
	for _, server := range o.Servers {
		if err := server.validate(); err != nil {
			return err
		}
	}

	for _, refclock := range o.Refclocks {
		if err := refclock.validate(); err != nil {
			return err
		}
	}

	return nil
}

// Use '*ToPtr()' functions from the internal/common package to set the pointer values in literals
type ChronyConfigServer struct {
	Hostname string `json:"hostname"`
	Minpoll  *int   `json:"minpoll,omitempty"`
	Maxpoll  *int   `json:"maxpoll,omitempty"`
	Iburst   *bool  `json:"iburst,omitempty"`
	Prefer   *bool  `json:"prefer,omitempty"`
}

func (s ChronyConfigServer) validate() error {
	if s.Hostname == "" {
		return fmt.Errorf("%s: server hostname is required", chronyConfStageType)
	}

	if minpoll := s.Minpoll; minpoll != nil && (*minpoll < -6 || *minpoll > 24) {
		return fmt.Errorf("%s: invalid server minpoll: must be in the range [-6, 24]", chronyConfStageType)
	}

	if maxpoll := s.Maxpoll; maxpoll != nil && (*maxpoll < -6 || *maxpoll > 24) {
		return fmt.Errorf("%s: invalid server maxpoll: must be in the range [-6, 24]", chronyConfStageType)
	}

	return nil
}

type ChronyConfigRefclock struct {
	Driver RefclockDriver `json:"driver"`
	Poll   *int           `json:"poll,omitempty"`
	Dpoll  *int           `json:"dpoll,omitempty"`
	Offset *float64       `json:"offset,omitempty"`
}

func (o ChronyConfigRefclock) validate() error {
	return o.Driver.validate()
}

type RefclockDriver interface {
	isRefclockDriver()
	validate() error
}

type ChronyDriverPPS struct {
	Name   string `json:"name"`
	Device string `json:"device"`
	Clear  *bool  `json:"clear,omitempty"`
}

func NewChronyDriverPPS(device string) ChronyDriverPPS {
	return ChronyDriverPPS{
		Name:   "PPS",
		Device: device,
	}
}

func (ChronyDriverPPS) isRefclockDriver() {}

func (p ChronyDriverPPS) validate() error {
	if p.Name != "PPS" {
		return fmt.Errorf("%s: invalid PPS driver name %q", chronyConfStageType, p.Name)
	}

	if err := validatePath(p.Device); err != nil {
		return fmt.Errorf("%s: invalid PPS device path: %w", chronyConfStageType, err)
	}

	return nil
}

type ChronyDriverSHM struct {
	Name    string  `json:"name"`
	Segment int     `json:"segment"`
	Perm    *string `json:"perm,omitempty"`
}

func NewChronyDriverSHM(segment int) ChronyDriverSHM {
	return ChronyDriverSHM{
		Name:    "SHM",
		Segment: segment,
	}
}

func (ChronyDriverSHM) isRefclockDriver() {}

func (s ChronyDriverSHM) validate() error {
	if s.Name != "SHM" {
		return fmt.Errorf("%s: invalid SHM driver name %q", chronyConfStageType, s.Name)
	}

	if perm := s.Perm; perm != nil {
		permRegex := regexp.MustCompile(chronyStagePermRegex)
		if !permRegex.MatchString(*perm) {
			return fmt.Errorf("%s: invalid SHM driver perm: %q doesn't match perm regular expression %q", chronyConfStageType, *perm, chronyStagePermRegex)
		}
	}
	return nil
}

type ChronyDriverSOCK struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func NewChronyDriverSOCK(path string) ChronyDriverSOCK {
	return ChronyDriverSOCK{
		Name: "SOCK",
		Path: path,
	}
}

func (ChronyDriverSOCK) isRefclockDriver() {}

func (s ChronyDriverSOCK) validate() error {
	if s.Name != "SOCK" {
		return fmt.Errorf("%s: invalid SOCK driver name %q", chronyConfStageType, s.Name)
	}

	if err := validatePath(s.Path); err != nil {
		return fmt.Errorf("%s: invalid SOCK socket path: %w", chronyConfStageType, err)
	}

	return nil
}

type ChronyDriverPHC struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Nocrossts *bool  `json:"nocrossts,omitempty"`
	Extpps    *bool  `json:"extpps,omitempty"`
	Pin       *int   `json:"pin,omitempty"`
	Channel   *int   `json:"channel,omitempty"`
	Clear     *bool  `json:"clear,omitempty"`
}

func NewChronyDriverPHC(path string) ChronyDriverPHC {
	return ChronyDriverPHC{
		Name: "PHC",
		Path: path,
	}
}

func (ChronyDriverPHC) isRefclockDriver() {}

func (p ChronyDriverPHC) validate() error {
	if p.Name != "PHC" {
		return fmt.Errorf("%s: invalid PHC driver name %q", chronyConfStageType, p.Name)
	}

	if err := validatePath(p.Path); err != nil {
		return fmt.Errorf("%s: invalid PHC path: %w", chronyConfStageType, err)
	}

	return nil
}

func NewChronyStage(options *ChronyStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    chronyConfStageType,
		Options: options,
	}
}

func validatePath(path string) error {
	invalidPathRegex := regexp.MustCompile(invalidPathRegex)
	if invalidPathRegex.FindAllString(path, -1) != nil {
		return fmt.Errorf("%q matches invalid path regular expression %q", path, invalidPathRegex)
	}

	return nil
}
