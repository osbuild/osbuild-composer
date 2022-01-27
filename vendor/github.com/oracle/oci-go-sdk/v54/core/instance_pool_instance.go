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

// InstancePoolInstance Information about an instance that belongs to an instance pool.
type InstancePoolInstance struct {

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the instance.
	Id *string `mandatory:"true" json:"id"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the instance pool.
	InstancePoolId *string `mandatory:"true" json:"instancePoolId"`

	// The availability domain the instance is running in.
	AvailabilityDomain *string `mandatory:"true" json:"availabilityDomain"`

	// The attachment state of the instance in relation to the instance pool.
	LifecycleState InstancePoolInstanceLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the compartment that contains the
	// instance.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the instance configuration
	// used to create the instance.
	InstanceConfigurationId *string `mandatory:"true" json:"instanceConfigurationId"`

	// The region that contains the availability domain the instance is running in.
	Region *string `mandatory:"true" json:"region"`

	// The shape of the instance. The shape determines the number of CPUs, amount of memory,
	// and other resources allocated to the instance.
	Shape *string `mandatory:"true" json:"shape"`

	// The lifecycle state of the instance. Refer to `lifecycleState` in the Instance resource.
	State *string `mandatory:"true" json:"state"`

	// The date and time the instance pool instance was created, in the format defined by RFC3339 (https://tools.ietf.org/html/rfc3339).
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// The fault domain the instance is running in.
	FaultDomain *string `mandatory:"false" json:"faultDomain"`

	// The load balancer backends that are configured for the instance.
	LoadBalancerBackends []InstancePoolInstanceLoadBalancerBackend `mandatory:"false" json:"loadBalancerBackends"`
}

func (m InstancePoolInstance) String() string {
	return common.PointerString(m)
}

// InstancePoolInstanceLifecycleStateEnum Enum with underlying type: string
type InstancePoolInstanceLifecycleStateEnum string

// Set of constants representing the allowable values for InstancePoolInstanceLifecycleStateEnum
const (
	InstancePoolInstanceLifecycleStateAttaching InstancePoolInstanceLifecycleStateEnum = "ATTACHING"
	InstancePoolInstanceLifecycleStateActive    InstancePoolInstanceLifecycleStateEnum = "ACTIVE"
	InstancePoolInstanceLifecycleStateDetaching InstancePoolInstanceLifecycleStateEnum = "DETACHING"
)

var mappingInstancePoolInstanceLifecycleState = map[string]InstancePoolInstanceLifecycleStateEnum{
	"ATTACHING": InstancePoolInstanceLifecycleStateAttaching,
	"ACTIVE":    InstancePoolInstanceLifecycleStateActive,
	"DETACHING": InstancePoolInstanceLifecycleStateDetaching,
}

// GetInstancePoolInstanceLifecycleStateEnumValues Enumerates the set of values for InstancePoolInstanceLifecycleStateEnum
func GetInstancePoolInstanceLifecycleStateEnumValues() []InstancePoolInstanceLifecycleStateEnum {
	values := make([]InstancePoolInstanceLifecycleStateEnum, 0)
	for _, v := range mappingInstancePoolInstanceLifecycleState {
		values = append(values, v)
	}
	return values
}
