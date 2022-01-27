// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"github.com/oracle/oci-go-sdk/v54/common"
)

// BulkDeleteResourcesDetails The representation of BulkDeleteResourcesDetails
type BulkDeleteResourcesDetails struct {

	// The resources to be deleted.
	Resources []BulkActionResource `mandatory:"true" json:"resources"`
}

func (m BulkDeleteResourcesDetails) String() string {
	return common.PointerString(m)
}
