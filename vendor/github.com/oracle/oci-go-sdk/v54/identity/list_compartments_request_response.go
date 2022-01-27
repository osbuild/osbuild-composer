// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package identity

import (
	"github.com/oracle/oci-go-sdk/v54/common"
	"net/http"
)

// ListCompartmentsRequest wrapper for the ListCompartments operation
//
// See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/identity/ListCompartments.go.html to see an example of how to use ListCompartmentsRequest.
type ListCompartmentsRequest struct {

	// The OCID of the compartment (remember that the tenancy is simply the root compartment).
	CompartmentId *string `mandatory:"true" contributesTo:"query" name:"compartmentId"`

	// The value of the `opc-next-page` response header from the previous "List" call.
	Page *string `mandatory:"false" contributesTo:"query" name:"page"`

	// The maximum number of items to return in a paginated "List" call.
	Limit *int `mandatory:"false" contributesTo:"query" name:"limit"`

	// Valid values are `ANY` and `ACCESSIBLE`. Default is `ANY`.
	// Setting this to `ACCESSIBLE` returns only those compartments for which the
	// user has INSPECT permissions directly or indirectly (permissions can be on a
	// resource in a subcompartment). For the compartments on which the user indirectly has
	// INSPECT permissions, a restricted set of fields is returned.
	// When set to `ANY` permissions are not checked.
	AccessLevel ListCompartmentsAccessLevelEnum `mandatory:"false" contributesTo:"query" name:"accessLevel" omitEmpty:"true"`

	// Default is false. Can only be set to true when performing
	// ListCompartments on the tenancy (root compartment).
	// When set to true, the hierarchy of compartments is traversed
	// and all compartments and subcompartments in the tenancy are
	// returned depending on the the setting of `accessLevel`.
	CompartmentIdInSubtree *bool `mandatory:"false" contributesTo:"query" name:"compartmentIdInSubtree"`

	// A filter to only return resources that match the given name exactly.
	Name *string `mandatory:"false" contributesTo:"query" name:"name"`

	// The field to sort by. You can provide one sort order (`sortOrder`). Default order for
	// TIMECREATED is descending. Default order for NAME is ascending. The NAME
	// sort order is case sensitive.
	// **Note:** In general, some "List" operations (for example, `ListInstances`) let you
	// optionally filter by Availability Domain if the scope of the resource type is within a
	// single Availability Domain. If you call one of these "List" operations without specifying
	// an Availability Domain, the resources are grouped by Availability Domain, then sorted.
	SortBy ListCompartmentsSortByEnum `mandatory:"false" contributesTo:"query" name:"sortBy" omitEmpty:"true"`

	// The sort order to use, either ascending (`ASC`) or descending (`DESC`). The NAME sort order
	// is case sensitive.
	SortOrder ListCompartmentsSortOrderEnum `mandatory:"false" contributesTo:"query" name:"sortOrder" omitEmpty:"true"`

	// A filter to only return resources that match the given lifecycle state.  The state value is case-insensitive.
	LifecycleState CompartmentLifecycleStateEnum `mandatory:"false" contributesTo:"query" name:"lifecycleState" omitEmpty:"true"`

	// Unique Oracle-assigned identifier for the request.
	// If you need to contact Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request ListCompartmentsRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request ListCompartmentsRequest) HTTPRequest(method, path string, binaryRequestBody *common.OCIReadSeekCloser, extraHeaders map[string]string) (http.Request, error) {

	return common.MakeDefaultHTTPRequestWithTaggedStructAndExtraHeaders(method, path, request, extraHeaders)
}

// BinaryRequestBody implements the OCIRequest interface
func (request ListCompartmentsRequest) BinaryRequestBody() (*common.OCIReadSeekCloser, bool) {

	return nil, false

}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request ListCompartmentsRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// ListCompartmentsResponse wrapper for the ListCompartments operation
type ListCompartmentsResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// A list of []Compartment instances
	Items []Compartment `presentIn:"body"`

	// Unique Oracle-assigned identifier for the request. If you need to contact Oracle about a
	// particular request, please provide the request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`

	// For pagination of a list of items. When paging through a list, if this header appears in the response,
	// then a partial list might have been returned. Include this value as the `page` parameter for the
	// subsequent GET request to get the next batch of items.
	OpcNextPage *string `presentIn:"header" name:"opc-next-page"`
}

func (response ListCompartmentsResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response ListCompartmentsResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// ListCompartmentsAccessLevelEnum Enum with underlying type: string
type ListCompartmentsAccessLevelEnum string

// Set of constants representing the allowable values for ListCompartmentsAccessLevelEnum
const (
	ListCompartmentsAccessLevelAny        ListCompartmentsAccessLevelEnum = "ANY"
	ListCompartmentsAccessLevelAccessible ListCompartmentsAccessLevelEnum = "ACCESSIBLE"
)

var mappingListCompartmentsAccessLevel = map[string]ListCompartmentsAccessLevelEnum{
	"ANY":        ListCompartmentsAccessLevelAny,
	"ACCESSIBLE": ListCompartmentsAccessLevelAccessible,
}

// GetListCompartmentsAccessLevelEnumValues Enumerates the set of values for ListCompartmentsAccessLevelEnum
func GetListCompartmentsAccessLevelEnumValues() []ListCompartmentsAccessLevelEnum {
	values := make([]ListCompartmentsAccessLevelEnum, 0)
	for _, v := range mappingListCompartmentsAccessLevel {
		values = append(values, v)
	}
	return values
}

// ListCompartmentsSortByEnum Enum with underlying type: string
type ListCompartmentsSortByEnum string

// Set of constants representing the allowable values for ListCompartmentsSortByEnum
const (
	ListCompartmentsSortByTimecreated ListCompartmentsSortByEnum = "TIMECREATED"
	ListCompartmentsSortByName        ListCompartmentsSortByEnum = "NAME"
)

var mappingListCompartmentsSortBy = map[string]ListCompartmentsSortByEnum{
	"TIMECREATED": ListCompartmentsSortByTimecreated,
	"NAME":        ListCompartmentsSortByName,
}

// GetListCompartmentsSortByEnumValues Enumerates the set of values for ListCompartmentsSortByEnum
func GetListCompartmentsSortByEnumValues() []ListCompartmentsSortByEnum {
	values := make([]ListCompartmentsSortByEnum, 0)
	for _, v := range mappingListCompartmentsSortBy {
		values = append(values, v)
	}
	return values
}

// ListCompartmentsSortOrderEnum Enum with underlying type: string
type ListCompartmentsSortOrderEnum string

// Set of constants representing the allowable values for ListCompartmentsSortOrderEnum
const (
	ListCompartmentsSortOrderAsc  ListCompartmentsSortOrderEnum = "ASC"
	ListCompartmentsSortOrderDesc ListCompartmentsSortOrderEnum = "DESC"
)

var mappingListCompartmentsSortOrder = map[string]ListCompartmentsSortOrderEnum{
	"ASC":  ListCompartmentsSortOrderAsc,
	"DESC": ListCompartmentsSortOrderDesc,
}

// GetListCompartmentsSortOrderEnumValues Enumerates the set of values for ListCompartmentsSortOrderEnum
func GetListCompartmentsSortOrderEnumValues() []ListCompartmentsSortOrderEnum {
	values := make([]ListCompartmentsSortOrderEnum, 0)
	for _, v := range mappingListCompartmentsSortOrder {
		values = append(values, v)
	}
	return values
}
