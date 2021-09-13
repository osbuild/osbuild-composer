package v2

import (
	"fmt"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestHTTPErrorReturnsEchoHTTPError(t *testing.T) {
	for _, se := range getServiceErrors() {
		err := HTTPError(se.code)
		echoError, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		require.Equal(t, se.httpStatus, echoError.Code)
		serviceErrorCode, ok := echoError.Message.(ServiceErrorCode)
		require.True(t, ok)
		require.Equal(t, se.code, serviceErrorCode)
	}
}

func TestAPIError(t *testing.T) {
	e := echo.New()
	for _, se := range getServiceErrors() {
		ctx := e.NewContext(nil, nil)
		ctx.Set("operationID", "test-operation-id")
		apiError := APIError(se.code, nil, ctx)
		require.Equal(t, fmt.Sprintf("/api/composer/v2/errors/%d", se.code), apiError.Href)
		require.Equal(t, fmt.Sprintf("%d", se.code), apiError.Id)
		require.Equal(t, "Error", apiError.Kind)
		require.Equal(t, fmt.Sprintf("COMPOSER-%d", se.code), apiError.Code)
		require.Equal(t, "test-operation-id", apiError.OperationId)
		require.Equal(t, se.reason, apiError.Reason)
	}
}

func TestAPIErrorOperationID(t *testing.T) {
	ctx := echo.New().NewContext(nil, nil)

	apiError := APIError(ErrorUnauthenticated, nil, ctx)
	require.Equal(t, "COMPOSER-10003", apiError.Code)

	ctx.Set("operationID", 5)
	apiError = APIError(ErrorUnauthenticated, nil, ctx)
	require.Equal(t, "COMPOSER-10003", apiError.Code)

	ctx.Set("operationID", "test-operation-id")
	apiError = APIError(ErrorUnauthenticated, nil, ctx)
	require.Equal(t, "COMPOSER-401", apiError.Code)
}

func TestAPIErrorList(t *testing.T) {
	ctx := echo.New().NewContext(nil, nil)
	ctx.Set("operationID", "test-operation-id")

	// negative values return empty list
	errs := APIErrorList(-10, -30, ctx)
	require.Equal(t, 0, errs.Size)
	require.Equal(t, 0, len(errs.Items))
	errs = APIErrorList(0, -30, ctx)
	require.Equal(t, 0, errs.Size)
	require.Equal(t, 0, len(errs.Items))
	errs = APIErrorList(-10, 0, ctx)
	require.Equal(t, 0, errs.Size)
	require.Equal(t, 0, len(errs.Items))

	// all of them
	errs = APIErrorList(0, 1000, ctx)
	require.Equal(t, len(getServiceErrors()), errs.Size)

	// some of them
	errs = APIErrorList(0, 10, ctx)
	require.Equal(t, 10, errs.Size)
	require.Equal(t, len(getServiceErrors()), errs.Total)
	require.Equal(t, 0, errs.Page)
	require.Equal(t, "COMPOSER-401", errs.Items[0].Code)
	errs = APIErrorList(1, 10, ctx)
	require.Equal(t, 10, errs.Size)
	require.Equal(t, len(getServiceErrors()), errs.Total)
	require.Equal(t, 1, errs.Page)
	require.Equal(t, "COMPOSER-11", errs.Items[0].Code)

	// high page
	errs = APIErrorList(1000, 1, ctx)
	require.Equal(t, 0, errs.Size)
	require.Equal(t, len(getServiceErrors()), errs.Total)
	require.Equal(t, 1000, errs.Page)

	// zero pagesize
	errs = APIErrorList(1000, 0, ctx)
	require.Equal(t, 0, errs.Size)
	require.Equal(t, len(getServiceErrors()), errs.Total)
	require.Equal(t, 1000, errs.Page)
}
