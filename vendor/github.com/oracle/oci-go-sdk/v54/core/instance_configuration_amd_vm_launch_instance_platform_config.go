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

// InstanceConfigurationAmdVmLaunchInstancePlatformConfig The platform configuration used when launching a virtual machine instance with the AMD platform.
type InstanceConfigurationAmdVmLaunchInstancePlatformConfig struct {

	// Whether Secure Boot is enabled on the instance.
	IsSecureBootEnabled *bool `mandatory:"false" json:"isSecureBootEnabled"`

	// Whether the Trusted Platform Module (TPM) is enabled on the instance.
	IsTrustedPlatformModuleEnabled *bool `mandatory:"false" json:"isTrustedPlatformModuleEnabled"`

	// Whether the Measured Boot feature is enabled on the instance.
	IsMeasuredBootEnabled *bool `mandatory:"false" json:"isMeasuredBootEnabled"`
}

//GetIsSecureBootEnabled returns IsSecureBootEnabled
func (m InstanceConfigurationAmdVmLaunchInstancePlatformConfig) GetIsSecureBootEnabled() *bool {
	return m.IsSecureBootEnabled
}

//GetIsTrustedPlatformModuleEnabled returns IsTrustedPlatformModuleEnabled
func (m InstanceConfigurationAmdVmLaunchInstancePlatformConfig) GetIsTrustedPlatformModuleEnabled() *bool {
	return m.IsTrustedPlatformModuleEnabled
}

//GetIsMeasuredBootEnabled returns IsMeasuredBootEnabled
func (m InstanceConfigurationAmdVmLaunchInstancePlatformConfig) GetIsMeasuredBootEnabled() *bool {
	return m.IsMeasuredBootEnabled
}

func (m InstanceConfigurationAmdVmLaunchInstancePlatformConfig) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m InstanceConfigurationAmdVmLaunchInstancePlatformConfig) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeInstanceConfigurationAmdVmLaunchInstancePlatformConfig InstanceConfigurationAmdVmLaunchInstancePlatformConfig
	s := struct {
		DiscriminatorParam string `json:"type"`
		MarshalTypeInstanceConfigurationAmdVmLaunchInstancePlatformConfig
	}{
		"AMD_VM",
		(MarshalTypeInstanceConfigurationAmdVmLaunchInstancePlatformConfig)(m),
	}

	return json.Marshal(&s)
}
