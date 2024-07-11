package osbuild

import "fmt"

const grubisoStageType = "org.osbuild.grub2.iso"

type GrubISOStageOptions struct {
	Product Product `json:"product"`

	Kernel ISOKernel `json:"kernel"`

	ISOLabel string `json:"isolabel"`

	Architectures []string `json:"architectures,omitempty"`

	Vendor string `json:"vendor,omitempty"`
}

func (GrubISOStageOptions) isStageOptions() {}

func (o GrubISOStageOptions) validate() error {
	// The stage schema marks product.name, product.version, kernel.dir, and
	// isolabel as required.  Empty values are technically valid according to
	// the schema, but here we will consider them invalid.

	if o.Product.Name == "" {
		return fmt.Errorf("%s: product.name option is required", grubisoStageType)
	}

	if o.Product.Version == "" {
		return fmt.Errorf("%s: product.version option is required", grubisoStageType)
	}

	if o.Kernel.Dir == "" {
		return fmt.Errorf("%s: kernel.dir option is required", grubisoStageType)
	}

	if o.ISOLabel == "" {
		return fmt.Errorf("%s: isolabel option is required", grubisoStageType)
	}

	return nil
}

type ISOKernel struct {
	Dir string `json:"dir"`

	// Additional kernel boot options
	Opts []string `json:"opts,omitempty"`
}

// Assemble a file system tree for a bootable ISO
func NewGrubISOStage(options *GrubISOStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    grubisoStageType,
		Options: options,
	}
}
