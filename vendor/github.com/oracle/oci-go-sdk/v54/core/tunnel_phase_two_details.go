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

// TunnelPhaseTwoDetails Tunnel detail information specific to IPSec phase 2.
type TunnelPhaseTwoDetails struct {

	// Indicates whether custom phase two configuration is enabled.
	IsCustomPhaseTwoConfig *bool `mandatory:"false" json:"isCustomPhaseTwoConfig"`

	// The total configured lifetime of an IKE security association.
	Lifetime *int64 `mandatory:"false" json:"lifetime"`

	// The lifetime remaining before the key is refreshed.
	RemainingLifetime *int64 `mandatory:"false" json:"remainingLifetime"`

	// Phase Two authentication algorithm supported during tunnel negotiation.
	CustomAuthenticationAlgorithm *string `mandatory:"false" json:"customAuthenticationAlgorithm"`

	// The negotiated authentication algorithm.
	NegotiatedAuthenticationAlgorithm *string `mandatory:"false" json:"negotiatedAuthenticationAlgorithm"`

	// Custom Encryption Algorithm
	CustomEncryptionAlgorithm *string `mandatory:"false" json:"customEncryptionAlgorithm"`

	// The negotiated encryption algorithm.
	NegotiatedEncryptionAlgorithm *string `mandatory:"false" json:"negotiatedEncryptionAlgorithm"`

	// Proposed Diffie-Hellman group.
	DhGroup *string `mandatory:"false" json:"dhGroup"`

	// The negotiated Diffie-Hellman group.
	NegotiatedDhGroup *string `mandatory:"false" json:"negotiatedDhGroup"`

	// ESP Phase 2 established
	IsEspEstablished *bool `mandatory:"false" json:"isEspEstablished"`

	// Is PFS (perfect forward secrecy) enabled
	IsPfsEnabled *bool `mandatory:"false" json:"isPfsEnabled"`

	// The date and time we retrieved the remaining lifetime, in the format defined by RFC3339 (https://tools.ietf.org/html/rfc3339).
	// Example: `2016-08-25T21:10:29.600Z`
	RemainingLifetimeLastRetrieved *common.SDKTime `mandatory:"false" json:"remainingLifetimeLastRetrieved"`
}

func (m TunnelPhaseTwoDetails) String() string {
	return common.PointerString(m)
}
