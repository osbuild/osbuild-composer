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
	"github.com/oracle/oci-go-sdk/v54/common"
)

// UpdateInstanceShapeConfigDetails The shape configuration requested for the instance. If provided, the instance will be updated
// with the resources specified. In the case where some properties are missing,
// the missing values will be set to the default for the provided `shape`.
// Each shape only supports certain configurable values. If the `shape` is provided
// and the configuration values are invalid for that new `shape`, an error will be returned.
// If no `shape` is provided and the configuration values are invalid for the instance's
// existing shape, an error will be returned.
type UpdateInstanceShapeConfigDetails struct {

	// The total number of OCPUs available to the instance.
	Ocpus *float32 `mandatory:"false" json:"ocpus"`

	// The total amount of memory available to the instance, in gigabytes.
	MemoryInGBs *float32 `mandatory:"false" json:"memoryInGBs"`

	// The baseline OCPU utilization for a subcore burstable VM instance. Leave this attribute blank for a
	// non-burstable instance, or explicitly specify non-burstable with `BASELINE_1_1`.
	// The following values are supported:
	// - `BASELINE_1_8` - baseline usage is 1/8 of an OCPU.
	// - `BASELINE_1_2` - baseline usage is 1/2 of an OCPU.
	// - `BASELINE_1_1` - baseline usage is an entire OCPU. This represents a non-burstable instance.
	BaselineOcpuUtilization UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum `mandatory:"false" json:"baselineOcpuUtilization,omitempty"`
}

func (m UpdateInstanceShapeConfigDetails) String() string {
	return common.PointerString(m)
}

// UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum Enum with underlying type: string
type UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum string

// Set of constants representing the allowable values for UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum
const (
	UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization8 UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum = "BASELINE_1_8"
	UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization2 UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum = "BASELINE_1_2"
	UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization1 UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum = "BASELINE_1_1"
)

var mappingUpdateInstanceShapeConfigDetailsBaselineOcpuUtilization = map[string]UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum{
	"BASELINE_1_8": UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization8,
	"BASELINE_1_2": UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization2,
	"BASELINE_1_1": UpdateInstanceShapeConfigDetailsBaselineOcpuUtilization1,
}

// GetUpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnumValues Enumerates the set of values for UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum
func GetUpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnumValues() []UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum {
	values := make([]UpdateInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum, 0)
	for _, v := range mappingUpdateInstanceShapeConfigDetailsBaselineOcpuUtilization {
		values = append(values, v)
	}
	return values
}
