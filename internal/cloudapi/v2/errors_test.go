package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		detailsError, ok := echoError.Message.(detailsError)
		require.True(t, ok)
		require.Equal(t, se.code, detailsError.errorCode)
	}
}

func TestAPIError(t *testing.T) {
	e := echo.New()
	for _, svcErr := range getServiceErrors() {
		ctx := e.NewContext(nil, nil)
		ctx.Set("operationID", "test-operation-id")
		se := svcErr // avoid G601
		apiError := APIError(&se, ctx, nil)
		require.Equal(t, fmt.Sprintf("/api/image-builder-composer/v2/errors/%d", se.code), apiError.Href)
		require.Equal(t, fmt.Sprintf("%d", se.code), apiError.Id)
		require.Equal(t, "Error", apiError.Kind)
		require.Equal(t, fmt.Sprintf("IMAGE-BUILDER-COMPOSER-%d", se.code), apiError.Code)
		require.Equal(t, "test-operation-id", apiError.OperationId)
		require.Equal(t, se.reason, apiError.Reason)
	}
}

func TestAPIErrorOperationID(t *testing.T) {
	ctx := echo.New().NewContext(nil, nil)

	apiError := APIError(find(ErrorUnauthenticated), ctx, nil)
	require.Equal(t, "IMAGE-BUILDER-COMPOSER-10003", apiError.Code)

	ctx.Set("operationID", 5)
	apiError = APIError(find(ErrorUnauthenticated), ctx, nil)
	require.Equal(t, "IMAGE-BUILDER-COMPOSER-10003", apiError.Code)

	ctx.Set("operationID", "test-operation-id")
	apiError = APIError(find(ErrorUnauthenticated), ctx, nil)
	require.Equal(t, "IMAGE-BUILDER-COMPOSER-401", apiError.Code)
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
	require.Equal(t, "IMAGE-BUILDER-COMPOSER-401", errs.Items[0].Code)
	errs = APIErrorList(1, 10, ctx)
	require.Equal(t, 10, errs.Size)
	require.Equal(t, len(getServiceErrors()), errs.Total)
	require.Equal(t, 1, errs.Page)
	require.Equal(t, "IMAGE-BUILDER-COMPOSER-11", errs.Items[0].Code)

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

func TestHTTPErrorHandler(t *testing.T) {
	e := echo.New()

	// HTTPError
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		HTTPErrorHandler(HTTPError(ErrorEnqueueingJob), c)
		require.Equal(t, find(ErrorEnqueueingJob).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorEnqueueingJob).reason, apiErr.Reason)
		require.Empty(t, *apiErr.Details)
	}

	// HTTPErrorWithInternal
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		err := fmt.Errorf("some more details")
		HTTPErrorHandler(HTTPErrorWithInternal(ErrorEnqueueingJob, err), c)
		require.Equal(t, find(ErrorEnqueueingJob).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorEnqueueingJob).reason, apiErr.Reason)
		require.Equal(t, err.Error(), *apiErr.Details)
	}

	// HTTPErrorWithDetails
	// internalErr gets ignored for explicit details
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		err := fmt.Errorf("some more details")
		HTTPErrorHandler(HTTPErrorWithDetails(ErrorEnqueueingJob, err, "even more extra details"), c)
		require.Equal(t, find(ErrorEnqueueingJob).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorEnqueueingJob).reason, apiErr.Reason)
		require.Equal(t, "even more extra details", *apiErr.Details)
	}

	// echo.HTTPError
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		err := fmt.Errorf("some unexpected internal http error")
		HTTPErrorHandler(echo.NewHTTPError(http.StatusInternalServerError, err), c)
		require.Equal(t, find(ErrorUnspecified).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorUnspecified).reason, apiErr.Reason)
		require.Equal(t, "code=500, message=some unexpected internal http error", *apiErr.Details)
	}

	// echo.HTTPError and internalErr is nil
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		HTTPErrorHandler(echo.NewHTTPError(http.StatusInternalServerError, nil), c)
		require.Equal(t, find(ErrorUnspecified).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorUnspecified).reason, apiErr.Reason)
		require.Equal(t, "code=500, message=<nil>", *apiErr.Details)
	}

	// plain error
	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("operationID", "opid")
		err := fmt.Errorf("some unexpected internal error")
		HTTPErrorHandler(err, c)
		require.Equal(t, find(ErrorNotHTTPError).httpStatus, rec.Code)
		var apiErr Error
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&apiErr))
		require.NotNil(t, apiErr)
		require.Equal(t, "opid", apiErr.OperationId)
		require.Equal(t, find(ErrorNotHTTPError).reason, apiErr.Reason)
		require.Equal(t, "some unexpected internal error", *apiErr.Details)
	}
}
