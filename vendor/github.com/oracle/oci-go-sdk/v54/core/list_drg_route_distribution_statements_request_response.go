// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package core

import (
	"github.com/oracle/oci-go-sdk/v54/common"
	"net/http"
)

// ListDrgRouteDistributionStatementsRequest wrapper for the ListDrgRouteDistributionStatements operation
//
// See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/core/ListDrgRouteDistributionStatements.go.html to see an example of how to use ListDrgRouteDistributionStatementsRequest.
type ListDrgRouteDistributionStatementsRequest struct {

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the route distribution.
	DrgRouteDistributionId *string `mandatory:"true" contributesTo:"path" name:"drgRouteDistributionId"`

	// For list pagination. The maximum number of results per page, or items to return in a paginated
	// "List" call. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	// Example: `50`
	Limit *int `mandatory:"false" contributesTo:"query" name:"limit"`

	// For list pagination. The value of the `opc-next-page` response header from the previous "List"
	// call. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	Page *string `mandatory:"false" contributesTo:"query" name:"page"`

	// The field to sort by.
	SortBy ListDrgRouteDistributionStatementsSortByEnum `mandatory:"false" contributesTo:"query" name:"sortBy" omitEmpty:"true"`

	// The sort order to use, either ascending (`ASC`) or descending (`DESC`). The DISPLAYNAME sort order
	// is case sensitive.
	SortOrder ListDrgRouteDistributionStatementsSortOrderEnum `mandatory:"false" contributesTo:"query" name:"sortOrder" omitEmpty:"true"`

	// Unique Oracle-assigned identifier for the request.
	// If you need to contact Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request ListDrgRouteDistributionStatementsRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request ListDrgRouteDistributionStatementsRequest) HTTPRequest(method, path string, binaryRequestBody *common.OCIReadSeekCloser, extraHeaders map[string]string) (http.Request, error) {

	return common.MakeDefaultHTTPRequestWithTaggedStructAndExtraHeaders(method, path, request, extraHeaders)
}

// BinaryRequestBody implements the OCIRequest interface
func (request ListDrgRouteDistributionStatementsRequest) BinaryRequestBody() (*common.OCIReadSeekCloser, bool) {

	return nil, false

}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request ListDrgRouteDistributionStatementsRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// ListDrgRouteDistributionStatementsResponse wrapper for the ListDrgRouteDistributionStatements operation
type ListDrgRouteDistributionStatementsResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// A list of []DrgRouteDistributionStatement instances
	Items []DrgRouteDistributionStatement `presentIn:"body"`

	// For list pagination. When this header appears in the response, additional pages
	// of results remain. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	OpcNextPage *string `presentIn:"header" name:"opc-next-page"`

	// Unique Oracle-assigned identifier for the request. If you need to contact
	// Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`
}

func (response ListDrgRouteDistributionStatementsResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response ListDrgRouteDistributionStatementsResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// ListDrgRouteDistributionStatementsSortByEnum Enum with underlying type: string
type ListDrgRouteDistributionStatementsSortByEnum string

// Set of constants representing the allowable values for ListDrgRouteDistributionStatementsSortByEnum
const (
	ListDrgRouteDistributionStatementsSortByTimecreated ListDrgRouteDistributionStatementsSortByEnum = "TIMECREATED"
)

var mappingListDrgRouteDistributionStatementsSortBy = map[string]ListDrgRouteDistributionStatementsSortByEnum{
	"TIMECREATED": ListDrgRouteDistributionStatementsSortByTimecreated,
}

// GetListDrgRouteDistributionStatementsSortByEnumValues Enumerates the set of values for ListDrgRouteDistributionStatementsSortByEnum
func GetListDrgRouteDistributionStatementsSortByEnumValues() []ListDrgRouteDistributionStatementsSortByEnum {
	values := make([]ListDrgRouteDistributionStatementsSortByEnum, 0)
	for _, v := range mappingListDrgRouteDistributionStatementsSortBy {
		values = append(values, v)
	}
	return values
}

// ListDrgRouteDistributionStatementsSortOrderEnum Enum with underlying type: string
type ListDrgRouteDistributionStatementsSortOrderEnum string

// Set of constants representing the allowable values for ListDrgRouteDistributionStatementsSortOrderEnum
const (
	ListDrgRouteDistributionStatementsSortOrderAsc  ListDrgRouteDistributionStatementsSortOrderEnum = "ASC"
	ListDrgRouteDistributionStatementsSortOrderDesc ListDrgRouteDistributionStatementsSortOrderEnum = "DESC"
)

var mappingListDrgRouteDistributionStatementsSortOrder = map[string]ListDrgRouteDistributionStatementsSortOrderEnum{
	"ASC":  ListDrgRouteDistributionStatementsSortOrderAsc,
	"DESC": ListDrgRouteDistributionStatementsSortOrderDesc,
}

// GetListDrgRouteDistributionStatementsSortOrderEnumValues Enumerates the set of values for ListDrgRouteDistributionStatementsSortOrderEnum
func GetListDrgRouteDistributionStatementsSortOrderEnumValues() []ListDrgRouteDistributionStatementsSortOrderEnum {
	values := make([]ListDrgRouteDistributionStatementsSortOrderEnum, 0)
	for _, v := range mappingListDrgRouteDistributionStatementsSortOrder {
		values = append(values, v)
	}
	return values
}
