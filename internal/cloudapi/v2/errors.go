package v2

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
)

const (
	ErrorCodePrefix = "IMAGE-BUILDER-COMPOSER-"
	ErrorHREF       = "/api/image-builder-composer/v2/errors"

	// ocm-sdk sends ErrorUnauthenticated with id 401 & code COMPOSER-401
	ErrorUnauthenticated ServiceErrorCode = 401

	ErrorUnauthorized            ServiceErrorCode = 2
	ErrorUnsupportedMediaType    ServiceErrorCode = 3
	ErrorUnsupportedDistribution ServiceErrorCode = 4
	ErrorUnsupportedArchitecture ServiceErrorCode = 5
	ErrorUnsupportedImageType    ServiceErrorCode = 6
	ErrorInvalidRepository       ServiceErrorCode = 7
	ErrorDNFError                ServiceErrorCode = 8
	ErrorInvalidOSTreeRef        ServiceErrorCode = 9
	ErrorInvalidOSTreeRepo       ServiceErrorCode = 10
	ErrorFailedToMakeManifest    ServiceErrorCode = 11
	ErrorInvalidComposeId        ServiceErrorCode = 14
	ErrorComposeNotFound         ServiceErrorCode = 15
	ErrorInvalidErrorId          ServiceErrorCode = 16
	ErrorErrorNotFound           ServiceErrorCode = 17
	ErrorInvalidPageParam        ServiceErrorCode = 18
	ErrorInvalidSizeParam        ServiceErrorCode = 19
	ErrorBodyDecodingError       ServiceErrorCode = 20
	ErrorResourceNotFound        ServiceErrorCode = 21
	ErrorMethodNotAllowed        ServiceErrorCode = 22
	ErrorNotAcceptable           ServiceErrorCode = 23

	// Internal errors, these are bugs
	ErrorFailedToInitializeBlueprint              ServiceErrorCode = 1000
	ErrorFailedToGenerateManifestSeed             ServiceErrorCode = 1001
	ErrorFailedToDepsolve                         ServiceErrorCode = 1002
	ErrorJSONMarshallingError                     ServiceErrorCode = 1003
	ErrorJSONUnMarshallingError                   ServiceErrorCode = 1004
	ErrorEnqueueingJob                            ServiceErrorCode = 1005
	ErrorSeveralUploadTargets                     ServiceErrorCode = 1006
	ErrorUnknownUploadTarget                      ServiceErrorCode = 1007
	ErrorFailedToLoadOpenAPISpec                  ServiceErrorCode = 1008
	ErrorFailedToParseManifestVersion             ServiceErrorCode = 1009
	ErrorUnknownManifestVersion                   ServiceErrorCode = 1010
	ErrorUnableToConvertOSTreeCommitStageMetadata ServiceErrorCode = 1011
	ErrorMalformedOSBuildJobResult                ServiceErrorCode = 1012
	ErrorGettingDepsolveJobStatus                 ServiceErrorCode = 1013
	ErrorDepsolveJobCanceled                      ServiceErrorCode = 1014

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

// Maps ServiceErrorcode to a reason and http code
func getServiceErrors() serviceErrors {
	return serviceErrors{
		serviceError{ErrorUnauthenticated, http.StatusUnauthorized, "Account authentication could not be verified"},
		serviceError{ErrorUnauthorized, http.StatusForbidden, "Account is unauthorized to perform this action"},
		serviceError{ErrorUnsupportedMediaType, http.StatusUnsupportedMediaType, "Only 'application/json' content is supported"},
		serviceError{ErrorUnsupportedDistribution, http.StatusBadRequest, "Unsupported distribution"},
		serviceError{ErrorUnsupportedArchitecture, http.StatusBadRequest, "Unsupported architecture"},
		serviceError{ErrorUnsupportedImageType, http.StatusBadRequest, "Unsupported image type"},
		serviceError{ErrorInvalidRepository, http.StatusBadRequest, "Must specify baseurl, mirrorlist, or metalink"},
		serviceError{ErrorDNFError, http.StatusBadRequest, "Failed to depsolve packages"},
		serviceError{ErrorInvalidOSTreeRef, http.StatusBadRequest, "Invalid OSTree ref"},
		serviceError{ErrorInvalidOSTreeRepo, http.StatusBadRequest, "Error resolving OSTree repo"},
		serviceError{ErrorFailedToMakeManifest, http.StatusBadRequest, "Failed to get manifest"},
		serviceError{ErrorInvalidComposeId, http.StatusBadRequest, "Invalid format for compose id"},
		serviceError{ErrorComposeNotFound, http.StatusNotFound, "Compose with given id not found"},
		serviceError{ErrorInvalidErrorId, http.StatusBadRequest, "Invalid format for error id, it should be an integer as a string"},
		serviceError{ErrorErrorNotFound, http.StatusNotFound, "Error with given id not found"},
		serviceError{ErrorInvalidPageParam, http.StatusBadRequest, "Invalid format for page param, it should be an integer as a string"},
		serviceError{ErrorInvalidSizeParam, http.StatusBadRequest, "Invalid format for size param, it should be an integer as a string"},
		serviceError{ErrorBodyDecodingError, http.StatusBadRequest, "Malformed json, unable to decode body"},
		serviceError{ErrorResourceNotFound, http.StatusNotFound, "Requested resource doesn't exist"},
		serviceError{ErrorMethodNotAllowed, http.StatusMethodNotAllowed, "Requested method isn't supported for resource"},
		serviceError{ErrorNotAcceptable, http.StatusNotAcceptable, "Only 'application/json' content is supported"},

		serviceError{ErrorFailedToInitializeBlueprint, http.StatusInternalServerError, "Failed to initialize blueprint"},
		serviceError{ErrorFailedToGenerateManifestSeed, http.StatusInternalServerError, "Failed to generate manifest seed"},
		serviceError{ErrorFailedToDepsolve, http.StatusInternalServerError, "Failed to depsolve packages"},
		serviceError{ErrorJSONMarshallingError, http.StatusInternalServerError, "Failed to marshal struct"},
		serviceError{ErrorJSONUnMarshallingError, http.StatusInternalServerError, "Failed to unmarshal struct"},
		serviceError{ErrorEnqueueingJob, http.StatusInternalServerError, "Failed to enqueue job"},
		serviceError{ErrorSeveralUploadTargets, http.StatusInternalServerError, "Compose has more than one upload target"},
		serviceError{ErrorUnknownUploadTarget, http.StatusInternalServerError, "Compose has unknown upload target"},
		serviceError{ErrorFailedToLoadOpenAPISpec, http.StatusInternalServerError, "Unable to load openapi spec"},
		serviceError{ErrorFailedToParseManifestVersion, http.StatusInternalServerError, "Unable to parse manifest version"},
		serviceError{ErrorUnknownManifestVersion, http.StatusInternalServerError, "Unknown manifest version"},
		serviceError{ErrorUnableToConvertOSTreeCommitStageMetadata, http.StatusInternalServerError, "Unable to convert ostree commit stage metadata"},
		serviceError{ErrorMalformedOSBuildJobResult, http.StatusInternalServerError, "OSBuildJobResult does not have expected fields set"},
		serviceError{ErrorGettingDepsolveJobStatus, http.StatusInternalServerError, "Unable to get depsolve job status"},
		serviceError{ErrorDepsolveJobCanceled, http.StatusInternalServerError, "Depsolve job was cancelled"},

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
		se = find(ErrorMalformedOperationID)
	}

	return &Error{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("%s/%d", ErrorHREF, se.code),
			Id:   fmt.Sprintf("%d", se.code),
			Kind: "Error",
		},
		Code:        fmt.Sprintf("%s%d", ErrorCodePrefix, se.code),
		OperationId: operationID, // set operation id from context
		Reason:      se.reason,
	}
}

// Helper to make the ErrorList as defined in openapi.v2.yml
func APIErrorList(page int, pageSize int, c echo.Context) *ErrorList {
	list := &ErrorList{
		List: List{
			Kind:  "ErrorList",
			Page:  page,
			Size:  0,
			Total: len(getServiceErrors()),
		},
		Items: []Error{},
	}

	if page < 0 || pageSize < 0 {
		return list
	}

	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	errs := getServiceErrors()[min(page*pageSize, len(getServiceErrors())):min(((page+1)*pageSize), len(getServiceErrors()))]
	for _, e := range errs {
		list.Items = append(list.Items, *APIError(e.code, &e, c))
	}
	list.Size = len(list.Items)
	return list
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
func (s *Server) HTTPErrorHandler(echoError error, c echo.Context) {
	doResponse := func(code ServiceErrorCode, c echo.Context) {
		if !c.Response().Committed {
			var err error
			sec := find(code)
			apiErr := APIError(code, sec, c)

			if sec.httpStatus == http.StatusInternalServerError {
				internalError, ok := echoError.(*echo.HTTPError)
				errMsg := fmt.Sprintf("Internal server error. Code: %s, OperationId: %s", apiErr.Code, apiErr.OperationId)

				if ok {
					errMsg += fmt.Sprintf(", InternalError: %v", internalError)
				}

				c.Logger().Error(errMsg)
			}

			if c.Request().Method == http.MethodHead {
				err = c.NoContent(sec.httpStatus)
			} else {
				err = c.JSON(sec.httpStatus, apiErr)
			}
			if err != nil {
				c.Logger().Errorf("Failed to return error response: %v", err)
			}
		} else {
			c.Logger().Infof("Failed to return error response, response already committed: %d", code)
		}
	}

	he, ok := echoError.(*echo.HTTPError)
	if !ok {
		c.Logger().Errorf("ErrorNotHTTPError %v", echoError)
		doResponse(ErrorNotHTTPError, c)
		return
	}

	internalError := he.Code >= http.StatusInternalServerError && he.Code <= http.StatusNetworkAuthenticationRequired
	if internalError {
		if strings.HasSuffix(c.Path(), "/compose") {
			prometheus.ComposeFailures.Inc()
		}
	}

	sec, ok := he.Message.(ServiceErrorCode)
	if !ok {
		// No service code was set, so Echo threw this error
		doResponse(apiErrorFromEchoError(he), c)
		return
	}
	doResponse(sec, c)
}
