package osbuild

import (
	"fmt"
	"slices"
)

type HMACAlgorithm string

const (
	HMACSHA1   = "sha1"
	HMACSHA224 = "sha224"
	HMACSHA256 = "sha256"
	HMACSHA384 = "sha384"
	HMACSHA512 = "sha512"
)

type HMACStageOptions struct {
	Paths     []string      `json:"paths"`
	Algorithm HMACAlgorithm `json:"algorithm"`
}

func (o *HMACStageOptions) isStageOptions() {}

func (o *HMACStageOptions) validate() error {
	if o == nil {
		return nil
	}
	if len(o.Paths) == 0 {
		return fmt.Errorf("'paths' is a required property")
	}
	if o.Algorithm == "" {
		return fmt.Errorf("'algorithm' is a required property")
	}

	algorithms := []HMACAlgorithm{
		HMACSHA1,
		HMACSHA224,
		HMACSHA256,
		HMACSHA384,
		HMACSHA512,
	}

	if !slices.Contains(algorithms, o.Algorithm) {
		return fmt.Errorf("'%s' is not one of %+v", o.Algorithm, algorithms)
	}

	return nil
}

func NewHMACStage(options *HMACStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.hmac",
		Options: options,
	}
}
