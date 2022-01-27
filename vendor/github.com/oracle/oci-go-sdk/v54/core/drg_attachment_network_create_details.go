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

// DrgAttachmentNetworkCreateDetails The representation of DrgAttachmentNetworkCreateDetails
type DrgAttachmentNetworkCreateDetails interface {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the network attached to the DRG.
	GetId() *string
}

type drgattachmentnetworkcreatedetails struct {
	JsonData []byte
	Id       *string `mandatory:"true" json:"id"`
	Type     string  `json:"type"`
}

// UnmarshalJSON unmarshals json
func (m *drgattachmentnetworkcreatedetails) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalerdrgattachmentnetworkcreatedetails drgattachmentnetworkcreatedetails
	s := struct {
		Model Unmarshalerdrgattachmentnetworkcreatedetails
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.Id = s.Model.Id
	m.Type = s.Model.Type

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *drgattachmentnetworkcreatedetails) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.Type {
	case "VCN":
		mm := VcnDrgAttachmentNetworkCreateDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

//GetId returns Id
func (m drgattachmentnetworkcreatedetails) GetId() *string {
	return m.Id
}

func (m drgattachmentnetworkcreatedetails) String() string {
	return common.PointerString(m)
}

// DrgAttachmentNetworkCreateDetailsTypeEnum Enum with underlying type: string
type DrgAttachmentNetworkCreateDetailsTypeEnum string

// Set of constants representing the allowable values for DrgAttachmentNetworkCreateDetailsTypeEnum
const (
	DrgAttachmentNetworkCreateDetailsTypeVcn DrgAttachmentNetworkCreateDetailsTypeEnum = "VCN"
)

var mappingDrgAttachmentNetworkCreateDetailsType = map[string]DrgAttachmentNetworkCreateDetailsTypeEnum{
	"VCN": DrgAttachmentNetworkCreateDetailsTypeVcn,
}

// GetDrgAttachmentNetworkCreateDetailsTypeEnumValues Enumerates the set of values for DrgAttachmentNetworkCreateDetailsTypeEnum
func GetDrgAttachmentNetworkCreateDetailsTypeEnumValues() []DrgAttachmentNetworkCreateDetailsTypeEnum {
	values := make([]DrgAttachmentNetworkCreateDetailsTypeEnum, 0)
	for _, v := range mappingDrgAttachmentNetworkCreateDetailsType {
		values = append(values, v)
	}
	return values
}
