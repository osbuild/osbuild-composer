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

// MeasuredBootEntry One Trusted Platform Module (TPM) Platform Configuration Register (PCR) entry. The entry might be measured during boot,
// or specified in a policy.
type MeasuredBootEntry struct {

	// The index of the policy.
	PcrIndex *string `mandatory:"false" json:"pcrIndex"`

	// The hashed PCR value.
	Value *string `mandatory:"false" json:"value"`

	// The type of algorithm used to calculate the hash.
	HashAlgorithm *string `mandatory:"false" json:"hashAlgorithm"`
}

func (m MeasuredBootEntry) String() string {
	return common.PointerString(m)
}
