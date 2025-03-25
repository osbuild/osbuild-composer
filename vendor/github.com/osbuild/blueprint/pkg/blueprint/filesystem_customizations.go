package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/pathpolicy"
)

type FilesystemCustomization struct {
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	MinSize    uint64 `json:"minsize,omitempty" toml:"minsize,omitempty"`

	// Note: The TOML `size` tag has been deprecated in favor of `minsize`.
	// we check for it in the TOML unmarshaler and use it as `minsize`.
	// However due to the TOML marshaler implementation, we can omit adding
	// a field for this tag and get the benifit of not having to export it.
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data interface{}) error {
	d, _ := data.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	var size uint64
	var minsize uint64

	// `size` is an alias for `minsize. We check for the `size` keyword
	// for backwards compatibility. We don't export a `Size` field as
	// we would like to discourage its use.
	switch d["size"].(type) {
	case int64:
		size = uint64(d["size"].(int64))
	case string:
		s, err := datasizes.Parse(d["size"].(string))
		if err != nil {
			return fmt.Errorf("TOML unmarshal: size is not valid filesystem size (%w)", err)
		}
		size = s
	case nil:
		size = 0
	default:
		return fmt.Errorf("TOML unmarshal: size must be integer or string, got %v of type %T", d["size"], d["size"])
	}

	switch d["minsize"].(type) {
	case int64:
		minsize = uint64(d["minsize"].(int64))
	case string:
		s, err := datasizes.Parse(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("TOML unmarshal: minsize is not valid filesystem size (%w)", err)
		}
		minsize = s
	case nil:
		minsize = 0
	default:
		return fmt.Errorf("TOML unmarshal: minsize must be integer or string, got %v of type %T", d["minsize"], d["minsize"])
	}

	if size == 0 && minsize == 0 {
		return fmt.Errorf("TOML unmarshal: minsize must be greater than 0, got %v", minsize)
	}

	if size > 0 && minsize == 0 {
		fsc.MinSize = size
		return nil
	}

	if size == 0 && minsize > 0 {
		fsc.MinSize = minsize
		return nil
	}

	if size > 0 && minsize > 0 {
		return fmt.Errorf("TOML unmarshal: size and minsize cannot both be set (size is an alias for minsize)")
	}

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
		return fmt.Errorf("JSON unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	// The JSON specification only mentions float64 and Go defaults to it: https://go.dev/blog/json
	switch d["minsize"].(type) {
	case float64:
		// Note that it uses different key than the TOML version
		fsc.MinSize = uint64(d["minsize"].(float64))
	case string:
		size, err := datasizes.Parse(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("JSON unmarshal: size is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = size
	default:
		return fmt.Errorf("JSON unmarshal: minsize must be float64 number or string, got %v of type %T", d["minsize"], d["minsize"])
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

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckMountpointsPolicy(mountpoints []FilesystemCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		err := mountpointAllowList.Check(m.Mountpoint)
		if err != nil {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	return nil
}
