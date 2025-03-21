package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	ErrorCodePrefix = "IMAGE-BUILDER-WORKER-"

	ErrorUnsupportedMediaType ServiceErrorCode = 3
	ErrorJobNotFound          ServiceErrorCode = 5
	ErrorJobNotRunning        ServiceErrorCode = 6
	ErrorMalformedJobId       ServiceErrorCode = 7
	ErrorMalformedJobToken    ServiceErrorCode = 8
	ErrorInvalidErrorId       ServiceErrorCode = 9
	ErrorBodyDecodingError    ServiceErrorCode = 10
	ErrorResourceNotFound     ServiceErrorCode = 11
	ErrorMethodNotAllowed     ServiceErrorCode = 12
	ErrorNotAcceptable        ServiceErrorCode = 13
	ErrorErrorNotFound        ServiceErrorCode = 14
	ErrorInvalidJobType       ServiceErrorCode = 15
	ErrorTenantNotFound       ServiceErrorCode = 16
	ErrorMalformedWorkerId    ServiceErrorCode = 17
	ErrorWorkerIdNotFound     ServiceErrorCode = 18

	// internal errors
	ErrorDiscardingArtifact       ServiceErrorCode = 1000
	ErrorCreatingArtifact         ServiceErrorCode = 1001
	ErrorWritingArtifact          ServiceErrorCode = 1002
	ErrorResolvingJobId           ServiceErrorCode = 1003
	ErrorFinishingJob             ServiceErrorCode = 1004
	ErrorRetrievingJobStatus      ServiceErrorCode = 1005
	ErrorRequestingJob            ServiceErrorCode = 1006
	ErrorFailedLoadingOpenAPISpec ServiceErrorCode = 1007
	ErrorInsertingWorker          ServiceErrorCode = 1008
	ErrorUpdatingWorkerStatus     ServiceErrorCode = 1009

	// Errors contained within this file
	ErrorUnspecified          ServiceErrorCode = 10000
	ErrorNotHTTPError         ServiceErrorCode = 10001
	ErrorServiceErrorNotFound ServiceErrorCode = 10002
	ErrorMalformedOperationID ServiceErrorCode = 10003
)

type ServiceErrorCode int

type serviceError struct {
	code       ServiceErrorCode
	httpStatus int
	reason     string
}

type serviceErrors []serviceError

func getServiceErrors() serviceErrors {
	return serviceErrors{
		serviceError{ErrorUnsupportedMediaType, http.StatusUnsupportedMediaType, "Only 'application/json' content is supported"},
		serviceError{ErrorBodyDecodingError, http.StatusBadRequest, "Malformed json, unable to decode body"},
		serviceError{ErrorJobNotFound, http.StatusNotFound, "Token not found"},
		serviceError{ErrorJobNotRunning, http.StatusBadRequest, "Job is not running"},
		serviceError{ErrorMalformedJobId, http.StatusBadRequest, "Given job id is not a uuidv4"},
		serviceError{ErrorMalformedJobToken, http.StatusBadRequest, "Given job id is not a uuidv4"},
		serviceError{ErrorInvalidErrorId, http.StatusBadRequest, "Invalid format for error id, it should be an integer as a string"},
		serviceError{ErrorResourceNotFound, http.StatusNotFound, "Requested resource doesn't exist"},
		serviceError{ErrorMethodNotAllowed, http.StatusMethodNotAllowed, "Requested method isn't supported for resource"},
		serviceError{ErrorNotAcceptable, http.StatusNotAcceptable, "Only 'application/json' content is supported"},
		serviceError{ErrorErrorNotFound, http.StatusNotFound, "Error with given id not found"},
		serviceError{ErrorInvalidJobType, http.StatusBadRequest, "Requested job type cannot be dequeued"},
		serviceError{ErrorTenantNotFound, http.StatusBadRequest, "Tenant not found in JWT claims"},
		serviceError{ErrorMalformedWorkerId, http.StatusBadRequest, "Given worker id is not a uuidv4"},
		serviceError{ErrorWorkerIdNotFound, http.StatusBadRequest, "Given worker id doesn't exist"},

		serviceError{ErrorDiscardingArtifact, http.StatusInternalServerError, "Error discarding artifact"},
		serviceError{ErrorCreatingArtifact, http.StatusInternalServerError, "Error creating artifact"},
		serviceError{ErrorWritingArtifact, http.StatusInternalServerError, "Error writing artifact"},
		serviceError{ErrorResolvingJobId, http.StatusInternalServerError, "Error resolving id from job token"},
		serviceError{ErrorFinishingJob, http.StatusInternalServerError, "Error finishing job"},
		serviceError{ErrorRetrievingJobStatus, http.StatusInternalServerError, "Error requesting job"},
		serviceError{ErrorRequestingJob, http.StatusInternalServerError, "Error requesting job"},
		serviceError{ErrorFailedLoadingOpenAPISpec, http.StatusInternalServerError, "Unable to load openapi spec"},
		serviceError{ErrorInsertingWorker, http.StatusInternalServerError, "Unable to register the worker"},
		serviceError{ErrorUpdatingWorkerStatus, http.StatusInternalServerError, "Unable update worker status"},

		serviceError{ErrorUnspecified, http.StatusInternalServerError, "Unspecified internal error "},
		serviceError{ErrorNotHTTPError, http.StatusInternalServerError, "Error is not an instance of HTTPError"},
		serviceError{ErrorServiceErrorNotFound, http.StatusInternalServerError, "Error does not exist"},
		serviceError{ErrorMalformedOperationID, http.StatusInternalServerError, "OperationID is empty or is not a string"},
	}
}

func find(code ServiceErrorCode) *serviceError {
	for _, e := range getServiceErrors() {
		if e.code == code {
			return &e
		}
	}
	return &serviceError{ErrorServiceErrorNotFound, http.StatusInternalServerError, "Error does not exist"}
}

// Make an echo compatible error out of a service error
func HTTPError(code ServiceErrorCode) error {
	return HTTPErrorWithInternal(code, nil)
}

// echo.HTTPError has a message interface{} field, which can be used to include the ServiceErrorCode
func HTTPErrorWithInternal(code ServiceErrorCode, internalErr error) error {
	se := find(code)
	he := echo.NewHTTPError(se.httpStatus, se.code)
	if internalErr != nil {
		he.Internal = internalErr
	}
	return he
}

// Convert a ServiceErrorCode into an Error as defined in openapi.v2.yml
// serviceError is optional, prevents multiple find() calls
func APIError(code ServiceErrorCode, serviceError *serviceError, c echo.Context) *Error {
	se := serviceError
	if se == nil {
		se = find(code)
	}

	operationID, ok := c.Get("operationID").(string)
	if !ok || operationID == "" {
		c.Logger().Errorf("Couldn't find operationID handling error %v", code)
		se = find(ErrorMalformedOperationID)
	}

	return &Error{
		Href:        fmt.Sprintf("%s/errors/%d", BasePath, se.code),
		Id:          fmt.Sprintf("%d", se.code),
		Kind:        "Error",
		Code:        fmt.Sprintf("%s%d", ErrorCodePrefix, se.code),
		OperationId: operationID, // set operation id from context
		Reason:      se.reason,
		Message:     se.reason, // backward compatibility
	}
}

func apiErrorFromEchoError(echoError *echo.HTTPError) ServiceErrorCode {
	switch echoError.Code {
	case http.StatusNotFound:
		return ErrorResourceNotFound
	case http.StatusMethodNotAllowed:
		return ErrorMethodNotAllowed
	case http.StatusNotAcceptable:
		return ErrorNotAcceptable
	default:
		return ErrorUnspecified
	}
}

// Convert an echo error into an AOC compliant one so we send a correct json error response
func HTTPErrorHandler(echoError error, c echo.Context) {
	doResponse := func(code ServiceErrorCode, c echo.Context, internal error) {
		if !c.Response().Committed {
			var err error
			sec := find(code)
			apiErr := APIError(code, sec, c)

			if sec.httpStatus == http.StatusInternalServerError {
				c.Logger().Errorf("Internal server error. Internal: %v, Code: %s, OperationId: %s",
					internal, apiErr.Code, apiErr.OperationId)
			} else {
				c.Logger().Infof("Code: %s, OperationId: %s, Internal: %v",
					apiErr.Code, apiErr.OperationId, internal)
			}

			if c.Request().Method == http.MethodHead {
				err = c.NoContent(sec.httpStatus)
			} else {
				err = c.JSON(sec.httpStatus, apiErr)
			}
			if err != nil {
				c.Logger().Errorf("Failed to return error response: %v", err)
			}
		}
	}

	he, ok := echoError.(*echo.HTTPError)
	if !ok {
		doResponse(ErrorNotHTTPError, c, echoError)
		return
	}

	sec, ok := he.Message.(ServiceErrorCode)
	if !ok {
		// No service code was set, so Echo threw this error
		doResponse(apiErrorFromEchoError(he), c, he.Internal)
		return
	}
	doResponse(sec, c, he.Internal)
}
