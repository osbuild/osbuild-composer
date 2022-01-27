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

// DrgAttachmentIdDrgRouteDistributionMatchCriteria The criteria by which a specific attachment will import routes to the DRG.
type DrgAttachmentIdDrgRouteDistributionMatchCriteria struct {

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the DRG attachment.
	DrgAttachmentId *string `mandatory:"true" json:"drgAttachmentId"`
}

func (m DrgAttachmentIdDrgRouteDistributionMatchCriteria) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m DrgAttachmentIdDrgRouteDistributionMatchCriteria) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeDrgAttachmentIdDrgRouteDistributionMatchCriteria DrgAttachmentIdDrgRouteDistributionMatchCriteria
	s := struct {
		DiscriminatorParam string `json:"matchType"`
		MarshalTypeDrgAttachmentIdDrgRouteDistributionMatchCriteria
	}{
		"DRG_ATTACHMENT_ID",
		(MarshalTypeDrgAttachmentIdDrgRouteDistributionMatchCriteria)(m),
	}

	return json.Marshal(&s)
}
