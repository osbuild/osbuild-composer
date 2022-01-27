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

// MacsecProperties Properties used for MACsec (if capable).
type MacsecProperties struct {

	// Indicates whether or not MACsec is enabled.
	State MacsecStateEnum `mandatory:"true" json:"state"`

	PrimaryKey *MacsecKey `mandatory:"false" json:"primaryKey"`

	// Type of encryption cipher suite to use for the MACsec connection.
	EncryptionCipher MacsecEncryptionCipherEnum `mandatory:"false" json:"encryptionCipher,omitempty"`
}

func (m MacsecProperties) String() string {
	return common.PointerString(m)
}
