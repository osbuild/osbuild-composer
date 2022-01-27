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

// DbCredential Database credentials are needed for onboarding cloud database to identity.
// The DB credentials are used for DB authentication with the service.
type DbCredential struct {

	// The OCID of the DB credential.
	Id *string `mandatory:"false" json:"id"`

	// The OCID of the user the DB credential belongs to.
	UserId *string `mandatory:"false" json:"userId"`

	// Date and time the `DbCredential` object was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`

	// Date and time when this credential will expire, in the format defined by RFC3339.
	// Null if it never expires.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeExpires *common.SDKTime `mandatory:"false" json:"timeExpires"`

	// The credential's current state. After creating a DB credential, make sure its `lifecycleState` changes from
	// CREATING to ACTIVE before using it.
	LifecycleState DbCredentialLifecycleStateEnum `mandatory:"false" json:"lifecycleState,omitempty"`

	// The detailed status of INACTIVE lifecycleState.
	LifecycleDetails *int64 `mandatory:"false" json:"lifecycleDetails"`
}

func (m DbCredential) String() string {
	return common.PointerString(m)
}

// DbCredentialLifecycleStateEnum Enum with underlying type: string
type DbCredentialLifecycleStateEnum string

// Set of constants representing the allowable values for DbCredentialLifecycleStateEnum
const (
	DbCredentialLifecycleStateCreating DbCredentialLifecycleStateEnum = "CREATING"
	DbCredentialLifecycleStateActive   DbCredentialLifecycleStateEnum = "ACTIVE"
	DbCredentialLifecycleStateDeleting DbCredentialLifecycleStateEnum = "DELETING"
	DbCredentialLifecycleStateDeleted  DbCredentialLifecycleStateEnum = "DELETED"
)

var mappingDbCredentialLifecycleState = map[string]DbCredentialLifecycleStateEnum{
	"CREATING": DbCredentialLifecycleStateCreating,
	"ACTIVE":   DbCredentialLifecycleStateActive,
	"DELETING": DbCredentialLifecycleStateDeleting,
	"DELETED":  DbCredentialLifecycleStateDeleted,
}

// GetDbCredentialLifecycleStateEnumValues Enumerates the set of values for DbCredentialLifecycleStateEnum
func GetDbCredentialLifecycleStateEnumValues() []DbCredentialLifecycleStateEnum {
	values := make([]DbCredentialLifecycleStateEnum, 0)
	for _, v := range mappingDbCredentialLifecycleState {
		values = append(values, v)
	}
	return values
}
