package osbuild

import "fmt"

const grub2isoLegacyStageType = "org.osbuild.grub2.iso.legacy"

type Grub2ISOLegacyStageOptions struct {
	Product Product `json:"product"`

	Kernel ISOKernel `json:"kernel"`

	ISOLabel string `json:"isolabel"`

	FIPS bool `json:"fips,omitempty"`
}

func (Grub2ISOLegacyStageOptions) isStageOptions() {}

func (o Grub2ISOLegacyStageOptions) validate() error {
	// The stage schema marks product.name, product.version, kernel.dir, and
	// isolabel as required.  Empty values are technically valid according to
	// the schema, but here we will consider them invalid.

	if o.Product.Name == "" {
		return fmt.Errorf("%s: product.name option is required", grub2isoLegacyStageType)
	}

	if o.Product.Version == "" {
		return fmt.Errorf("%s: product.version option is required", grub2isoLegacyStageType)
	}

	if o.Kernel.Dir == "" {
		return fmt.Errorf("%s: kernel.dir option is required", grub2isoLegacyStageType)
	}

	if o.ISOLabel == "" {
		return fmt.Errorf("%s: isolabel option is required", grub2isoLegacyStageType)
	}

	return nil
}

// Assemble a file system tree for a bootable ISO
func NewGrub2ISOLegacyStage(options *Grub2ISOLegacyStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    grub2isoLegacyStageType,
		Options: options,
	}
}
