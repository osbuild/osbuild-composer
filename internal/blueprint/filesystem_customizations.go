package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/common"
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
		sizeInt := d["size"].(int64)
		if sizeInt < 0 {
			return fmt.Errorf("TOML unmarshal: size cannot be negative: %d", sizeInt)
		}
		size = uint64(sizeInt)
	case string:
		s, err := common.DataSizeToUint64(d["size"].(string))
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
		sizeInt := d["minsize"].(int64)
		if sizeInt < 0 {
			return fmt.Errorf("TOML unmarshal: minsize cannot be negative: %d", sizeInt)
		}
		minsize = uint64(sizeInt)
	case string:
		s, err := common.DataSizeToUint64(d["minsize"].(string))
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
		size, err := common.DataSizeToUint64(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("JSON unmarshal: size is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = size
	default:
		return fmt.Errorf("JSON unmarshal: minsize must be float64 number or string, got %v of type %T", d["minsize"], d["minsize"])
	}

	return nil
}
