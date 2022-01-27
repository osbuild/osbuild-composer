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

// Vcn A virtual cloud network (VCN). For more information, see
// Overview of the Networking Service (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/overview.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/iaas/Content/Identity/Concepts/policygetstarted.htm).
type Vcn struct {

	// Deprecated. The first CIDR IP address from cidrBlocks.
	// Example: `172.16.0.0/16`
	CidrBlock *string `mandatory:"true" json:"cidrBlock"`

	// The list of IPv4 CIDR blocks the VCN will use.
	CidrBlocks []string `mandatory:"true" json:"cidrBlocks"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the compartment containing the VCN.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The VCN's Oracle ID (OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm)).
	Id *string `mandatory:"true" json:"id"`

	// The VCN's current state.
	LifecycleState VcnLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) for the VCN's default set of DHCP options.
	DefaultDhcpOptionsId *string `mandatory:"false" json:"defaultDhcpOptionsId"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) for the VCN's default route table.
	DefaultRouteTableId *string `mandatory:"false" json:"defaultRouteTableId"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) for the VCN's default security list.
	DefaultSecurityListId *string `mandatory:"false" json:"defaultSecurityListId"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// A DNS label for the VCN, used in conjunction with the VNIC's hostname and
	// subnet's DNS label to form a fully qualified domain name (FQDN) for each VNIC
	// within this subnet (for example, `bminstance-1.subnet123.vcn1.oraclevcn.com`).
	// Must be an alphanumeric string that begins with a letter.
	// The value cannot be changed.
	// The absence of this parameter means the Internet and VCN Resolver will
	// not work for this VCN.
	// For more information, see
	// DNS in Your Virtual Cloud Network (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/dns.htm).
	// Example: `vcn1`
	DnsLabel *string `mandatory:"false" json:"dnsLabel"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// For an IPv6-enabled VCN, this is the list of IPv6 CIDR blocks for the VCN's IP address space.
	// The CIDRs are provided by Oracle and the sizes are always /56.
	Ipv6CidrBlocks []string `mandatory:"false" json:"ipv6CidrBlocks"`

	// The date and time the VCN was created, in the format defined by RFC3339 (https://tools.ietf.org/html/rfc3339).
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`

	// The VCN's domain name, which consists of the VCN's DNS label, and the
	// `oraclevcn.com` domain.
	// For more information, see
	// DNS in Your Virtual Cloud Network (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/dns.htm).
	// Example: `vcn1.oraclevcn.com`
	VcnDomainName *string `mandatory:"false" json:"vcnDomainName"`
}

func (m Vcn) String() string {
	return common.PointerString(m)
}

// VcnLifecycleStateEnum Enum with underlying type: string
type VcnLifecycleStateEnum string

// Set of constants representing the allowable values for VcnLifecycleStateEnum
const (
	VcnLifecycleStateProvisioning VcnLifecycleStateEnum = "PROVISIONING"
	VcnLifecycleStateAvailable    VcnLifecycleStateEnum = "AVAILABLE"
	VcnLifecycleStateTerminating  VcnLifecycleStateEnum = "TERMINATING"
	VcnLifecycleStateTerminated   VcnLifecycleStateEnum = "TERMINATED"
	VcnLifecycleStateUpdating     VcnLifecycleStateEnum = "UPDATING"
)

var mappingVcnLifecycleState = map[string]VcnLifecycleStateEnum{
	"PROVISIONING": VcnLifecycleStateProvisioning,
	"AVAILABLE":    VcnLifecycleStateAvailable,
	"TERMINATING":  VcnLifecycleStateTerminating,
	"TERMINATED":   VcnLifecycleStateTerminated,
	"UPDATING":     VcnLifecycleStateUpdating,
}

// GetVcnLifecycleStateEnumValues Enumerates the set of values for VcnLifecycleStateEnum
func GetVcnLifecycleStateEnumValues() []VcnLifecycleStateEnum {
	values := make([]VcnLifecycleStateEnum, 0)
	for _, v := range mappingVcnLifecycleState {
		values = append(values, v)
	}
	return values
}
