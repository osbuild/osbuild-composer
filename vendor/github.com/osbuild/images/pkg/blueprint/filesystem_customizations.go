package blueprint

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/pathpolicy"
)

type FilesystemCustomization struct {
	Mountpoint string
	MinSize    uint64
}

type filesystemCustomizationMarshaling struct {
	Mountpoint string         `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	MinSize    datasizes.Size `json:"minsize,omitempty" toml:"minsize,omitempty"`
}

func (fsc *FilesystemCustomization) UnmarshalJSON(data []byte) error {
	var fc filesystemCustomizationMarshaling
	if err := json.Unmarshal(data, &fc); err != nil {
		if fc.Mountpoint != "" {
			return fmt.Errorf("error decoding minsize value for mountpoint %q: %w", fc.Mountpoint, err)
		}
		return err
	}
	fsc.Mountpoint = fc.Mountpoint
	fsc.MinSize = fc.MinSize.Uint64()

	return nil
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data any) error {
	return unmarshalTOMLviaJSON(fsc, data)
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
