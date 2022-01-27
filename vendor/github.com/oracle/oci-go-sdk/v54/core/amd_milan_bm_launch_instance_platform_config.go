// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Core Services API
//
// Use the Core Services API to manage resources such as virtual cloud networks (VCNs),
// compute instances, and block storage volumes. For more information, see the console
// documentation for the Networking (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/overview.htm),
// Compute (https://docs.cloud.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm), and
// Block Volume (https://docs.cloud.oracle.com/iaas/Content/Block/Concepts/overview.htm) services.
//

package core

import (
	"encoding/json"
	"github.com/oracle/oci-go-sdk/v54/common"
)

// AmdMilanBmLaunchInstancePlatformConfig The platform configuration used when launching a bare metal instance with an E4 shape
// (the AMD Milan platform).
type AmdMilanBmLaunchInstancePlatformConfig struct {

	// Whether Secure Boot is enabled on the instance.
	IsSecureBootEnabled *bool `mandatory:"false" json:"isSecureBootEnabled"`

	// Whether the Trusted Platform Module (TPM) is enabled on the instance.
	IsTrustedPlatformModuleEnabled *bool `mandatory:"false" json:"isTrustedPlatformModuleEnabled"`

	// Whether the Measured Boot feature is enabled on the instance.
	IsMeasuredBootEnabled *bool `mandatory:"false" json:"isMeasuredBootEnabled"`

	// The number of NUMA nodes per socket (NPS).
	NumaNodesPerSocket AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum `mandatory:"false" json:"numaNodesPerSocket,omitempty"`
}

//GetIsSecureBootEnabled returns IsSecureBootEnabled
func (m AmdMilanBmLaunchInstancePlatformConfig) GetIsSecureBootEnabled() *bool {
	return m.IsSecureBootEnabled
}

//GetIsTrustedPlatformModuleEnabled returns IsTrustedPlatformModuleEnabled
func (m AmdMilanBmLaunchInstancePlatformConfig) GetIsTrustedPlatformModuleEnabled() *bool {
	return m.IsTrustedPlatformModuleEnabled
}

//GetIsMeasuredBootEnabled returns IsMeasuredBootEnabled
func (m AmdMilanBmLaunchInstancePlatformConfig) GetIsMeasuredBootEnabled() *bool {
	return m.IsMeasuredBootEnabled
}

func (m AmdMilanBmLaunchInstancePlatformConfig) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m AmdMilanBmLaunchInstancePlatformConfig) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeAmdMilanBmLaunchInstancePlatformConfig AmdMilanBmLaunchInstancePlatformConfig
	s := struct {
		DiscriminatorParam string `json:"type"`
		MarshalTypeAmdMilanBmLaunchInstancePlatformConfig
	}{
		"AMD_MILAN_BM",
		(MarshalTypeAmdMilanBmLaunchInstancePlatformConfig)(m),
	}

	return json.Marshal(&s)
}

// AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum Enum with underlying type: string
type AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum string

// Set of constants representing the allowable values for AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum
const (
	AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps0 AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum = "NPS0"
	AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps1 AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum = "NPS1"
	AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps2 AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum = "NPS2"
	AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps4 AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum = "NPS4"
)

var mappingAmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocket = map[string]AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum{
	"NPS0": AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps0,
	"NPS1": AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps1,
	"NPS2": AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps2,
	"NPS4": AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketNps4,
}

// GetAmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnumValues Enumerates the set of values for AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum
func GetAmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnumValues() []AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum {
	values := make([]AmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocketEnum, 0)
	for _, v := range mappingAmdMilanBmLaunchInstancePlatformConfigNumaNodesPerSocket {
		values = append(values, v)
	}
	return values
}
