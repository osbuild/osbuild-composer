package blueprint

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/pathpolicy"
)

type FilesystemCustomization struct {
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	MinSize    uint64 `json:"minsize,omitempty" toml:"minsize,omitempty"`
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data interface{}) error {
	d, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("customizations.filesystem is not an object")
	}

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: mountpoint must be string, got \"%v\" of type %T", d["mountpoint"], d["mountpoint"])
	}
	minSize, err := decodeSize(d["minsize"])
	if err != nil {
		return fmt.Errorf("TOML unmarshal: error decoding minsize value for mountpoint %q: %w", fsc.Mountpoint, err)
	}
	fsc.MinSize = minSize
	return nil
}

func (fsc *FilesystemCustomization) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	d, _ := v.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("JSON unmarshal: mountpoint must be string, got \"%v\" of type %T", d["mountpoint"], d["mountpoint"])
	}

	minSize, err := decodeSize(d["minsize"])
	if err != nil {
		return fmt.Errorf("JSON unmarshal: error decoding minsize value for mountpoint %q: %w", fsc.Mountpoint, err)
	}
	fsc.MinSize = minSize
	return nil
}

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckMountpointsPolicy(mountpoints []FilesystemCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	var errs []error
	for _, m := range mountpoints {
		if err := mountpointAllowList.Check(m.Mountpoint); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while setting up custom mountpoints:\n%w", errors.Join(errs...))
	}

	return nil
}

// decodeSize takes an integer or string representing a data size (with a data
// suffix) and returns the uint64 representation.
func decodeSize(size any) (uint64, error) {
	switch s := size.(type) {
	case string:
		return datasizes.Parse(s)
	case int64:
		if s < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		return uint64(s), nil
	case float64:
		if s < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		// TODO: emit warning of possible truncation?
		return uint64(s), nil
	case uint64:
		return s, nil
	default:
		return 0, fmt.Errorf("failed to convert value \"%v\" to number", size)
	}
}
