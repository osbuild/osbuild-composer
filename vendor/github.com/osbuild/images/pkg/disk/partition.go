package disk

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/internal/common"
)

type Partition struct {
	// Start of the partition in bytes
	Start uint64 `json:"start,omitempty" yaml:"start,omitempty"`
	// Size of the partition in bytes
	Size uint64 `json:"size" yaml:"size"`
	// Partition type, e.g. 0x83 for MBR or a UUID for gpt
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	// `Legacy BIOS bootable` (GPT) or `active` (DOS) flag
	Bootable bool `json:"bootable,omitempty" yaml:"bootable,omitempty"`

	// ID of the partition, dos doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID string `json:"uuid,omitempty" yaml:"uuid,omitempty"`

	// Partition name (not filesystem label), only supported for GPT
	Label string `json:"label,omitempty" yaml:"label,omitempty"`

	// If nil, the partition is raw; It doesn't contain a payload.
	Payload PayloadEntity `json:"payload,omitempty" yaml:"payload,omitempty"`
}

func (p *Partition) Clone() Entity {
	if p == nil {
		return nil
	}

	partition := &Partition{
		Start:    p.Start,
		Size:     p.Size,
		Type:     p.Type,
		Bootable: p.Bootable,
		UUID:     p.UUID,
		Label:    p.Label,
	}

	if p.Payload != nil {
		partition.Payload = p.Payload.Clone().(PayloadEntity)
	}

	return partition
}

func (pt *Partition) GetItemCount() uint {
	if pt == nil || pt.Payload == nil {
		return 0
	}
	return 1
}

func (p *Partition) GetChild(n uint) Entity {
	if n != 0 {
		panic(fmt.Sprintf("invalid child index for Partition: %d != 0", n))
	}
	return p.Payload
}

// fitTo resizes a partition to be either the given the size or the size of its
// payload if that is larger. The payload can be larger if it is a container
// with sized children.
func (p *Partition) fitTo(size uint64) {
	payload, isVC := p.Payload.(VolumeContainer)
	if isVC {
		if payloadMinSize := payload.minSize(size); payloadMinSize > size {
			size = payloadMinSize
		}
	}
	p.Size = size
}

func (p *Partition) GetSize() uint64 {
	return p.Size
}

// Ensure the partition has at least the given size. Will do nothing
// if the partition is already larger. Returns if the size changed.
func (p *Partition) EnsureSize(s uint64) bool {
	if s > p.Size {
		p.Size = s
		return true
	}
	return false
}

func (p *Partition) IsBIOSBoot() bool {
	if p == nil {
		return false
	}

	return p.Type == BIOSBootPartitionGUID || p.Type == BIOSBootPartitionDOSID
}

func (p *Partition) IsPReP() bool {
	if p == nil {
		return false
	}

	return p.Type == PRepPartitionDOSID || p.Type == PRePartitionGUID
}

func (p *Partition) MarshalJSON() ([]byte, error) {
	type partAlias Partition

	var entityName string
	if p.Payload != nil {
		entityName = p.Payload.EntityName()
	}

	partWithPayloadType := struct {
		partAlias
		PayloadType string `json:"payload_type,omitempty" yaml:"payload_type,omitempty"`
	}{
		partAlias(*p),
		entityName,
	}

	return json.Marshal(partWithPayloadType)
}

func (p *Partition) UnmarshalJSON(data []byte) (err error) {
	// keep in sync with lvm.go,partition.go,luks.go
	type alias Partition
	var withoutPayload struct {
		alias
		Payload     json.RawMessage `json:"payload" yaml:"payload"`
		PayloadType string          `json:"payload_type" yaml:"payload_type"`
	}
	if err := jsonUnmarshalStrict(data, &withoutPayload); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", data, err)
	}
	*p = Partition(withoutPayload.alias)

	p.Payload, err = unmarshalJSONPayload(data)
	return err
}

func (t *Partition) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(t, unmarshal)
}
