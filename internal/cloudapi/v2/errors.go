package v2

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	ErrorCodePrefix = "IMAGE-BUILDER-COMPOSER-"
	ErrorHREF       = "/api/image-builder-composer/v2/errors"

	// ocm-sdk sends ErrorUnauthenticated with id 401 & code COMPOSER-401
	ErrorUnauthenticated ServiceErrorCode = 401

	ErrorUnauthorized                 ServiceErrorCode = 2
	ErrorUnsupportedMediaType         ServiceErrorCode = 3
	ErrorUnsupportedDistribution      ServiceErrorCode = 4
	ErrorUnsupportedArchitecture      ServiceErrorCode = 5
	ErrorUnsupportedImageType         ServiceErrorCode = 6
	ErrorInvalidRepository            ServiceErrorCode = 7
	ErrorDNFError                     ServiceErrorCode = 8
	ErrorInvalidOSTreeRef             ServiceErrorCode = 9
	ErrorInvalidOSTreeRepo            ServiceErrorCode = 10
	ErrorFailedToMakeManifest         ServiceErrorCode = 11
	ErrorInvalidComposeId             ServiceErrorCode = 14
	ErrorComposeNotFound              ServiceErrorCode = 15
	ErrorInvalidErrorId               ServiceErrorCode = 16
	ErrorErrorNotFound                ServiceErrorCode = 17
	ErrorInvalidPageParam             ServiceErrorCode = 18
	ErrorInvalidSizeParam             ServiceErrorCode = 19
	ErrorBodyDecodingError            ServiceErrorCode = 20
	ErrorResourceNotFound             ServiceErrorCode = 21
	ErrorMethodNotAllowed             ServiceErrorCode = 22
	ErrorNotAcceptable                ServiceErrorCode = 23
	ErrorNoBaseURLInPayloadRepository ServiceErrorCode = 24
	ErrorInvalidNumberOfImageBuilds   ServiceErrorCode = 25
	ErrorInvalidJobType               ServiceErrorCode = 26
	ErrorInvalidOSTreeParams          ServiceErrorCode = 27
	ErrorTenantNotFound               ServiceErrorCode = 28
	ErrorNoGPGKey                     ServiceErrorCode = 29
	ErrorValidationFailed             ServiceErrorCode = 30
	ErrorComposeBadState              ServiceErrorCode = 31
	ErrorUnsupportedImage             ServiceErrorCode = 32
	ErrorInvalidImageFromComposeId    ServiceErrorCode = 33
	ErrorImageNotFound                ServiceErrorCode = 34
	ErrorInvalidCustomization         ServiceErrorCode = 35
	ErrorLocalSaveNotEnabled          ServiceErrorCode = 36
	ErrorInvalidPartitioningMode      ServiceErrorCode = 37
	ErrorInvalidUploadTarget          ServiceErrorCode = 38
	ErrorBlueprintOrCustomNotBoth     ServiceErrorCode = 39
	ErrorMismatchedDistribution       ServiceErrorCode = 40
	ErrorMismatchedArchitecture       ServiceErrorCode = 41
	ErrorBadRequest                   ServiceErrorCode = 42

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
	ErrorUnexpectedNumberOfImageBuilds            ServiceErrorCode = 1015
	ErrorGettingBuildDependencyStatus             ServiceErrorCode = 1016
	ErrorGettingOSBuildJobStatus                  ServiceErrorCode = 1017
	ErrorGettingAWSEC2JobStatus                   ServiceErrorCode = 1018
	ErrorGettingJobType                           ServiceErrorCode = 1019
	ErrorTenantNotInContext                       ServiceErrorCode = 1020
	ErrorGettingComposeList                       ServiceErrorCode = 1021
	ErrorArtifactNotFound                         ServiceErrorCode = 1022
	ErrorDeletingJob                              ServiceErrorCode = 1023

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
		serviceError{ErrorNoBaseURLInPayloadRepository, http.StatusBadRequest, "BaseURL must be specified for payload repositories"},
		serviceError{ErrorInvalidJobType, http.StatusNotFound, "Job with given id has an invalid type"},
		serviceError{ErrorInvalidNumberOfImageBuilds, http.StatusBadRequest, "Compose request has unsupported number of image builds"},
		serviceError{ErrorInvalidOSTreeParams, http.StatusBadRequest, "Invalid OSTree parameters or parameter combination"},
		serviceError{ErrorTenantNotFound, http.StatusBadRequest, "Tenant not found in JWT claims"},
		serviceError{ErrorArtifactNotFound, http.StatusBadRequest, "Artifact not found"},
		serviceError{ErrorNoGPGKey, http.StatusBadRequest, "Invalid repository, when check_gpg is set, gpgkey must be specified"},
		serviceError{ErrorValidationFailed, http.StatusBadRequest, "Request could not be validated"},
		serviceError{ErrorComposeBadState, http.StatusBadRequest, "Compose is running or has failed"},
		serviceError{ErrorUnsupportedImage, http.StatusBadRequest, "This compose doesn't support the creation of multiple images"},
		serviceError{ErrorInvalidImageFromComposeId, http.StatusBadRequest, "Invalid format for image id"},
		serviceError{ErrorImageNotFound, http.StatusBadRequest, "Image with given id not found"},
		serviceError{ErrorInvalidCustomization, http.StatusBadRequest, "Invalid image customization"},
		serviceError{ErrorLocalSaveNotEnabled, http.StatusBadRequest, "local_save is not enabled"},
		serviceError{ErrorInvalidPartitioningMode, http.StatusBadRequest, "Requested partitioning mode is invalid"},
		serviceError{ErrorInvalidUploadTarget, http.StatusBadRequest, "Invalid upload target for image type"},
		serviceError{ErrorBlueprintOrCustomNotBoth, http.StatusBadRequest, "Invalid request, include blueprint or customizations, not both"},
		serviceError{ErrorMismatchedDistribution, http.StatusBadRequest, "Invalid request, Blueprint and Cloud API request Distribution must match"},
		serviceError{ErrorMismatchedArchitecture, http.StatusBadRequest, "Invalid request, Blueprint and Cloud API request Architecture must match"},
		serviceError{ErrorBadRequest, http.StatusBadRequest, "Invalid request, see details for more information"},

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
		serviceError{ErrorUnexpectedNumberOfImageBuilds, http.StatusInternalServerError, "Compose has unexpected number of image builds"},
		serviceError{ErrorGettingBuildDependencyStatus, http.StatusInternalServerError, "Error checking status of build job dependencies"},
		serviceError{ErrorGettingOSBuildJobStatus, http.StatusInternalServerError, "Unable to get osbuild job status"},
		serviceError{ErrorGettingAWSEC2JobStatus, http.StatusInternalServerError, "Unable to get ec2 job status"},
		serviceError{ErrorGettingJobType, http.StatusInternalServerError, "Unable to get job type of existing job"},
		serviceError{ErrorTenantNotInContext, http.StatusInternalServerError, "Unable to retrieve tenant from request context"},
		serviceError{ErrorGettingComposeList, http.StatusInternalServerError, "Unable to get list of composes"},
		serviceError{ErrorDeletingJob, http.StatusInternalServerError, "Unable to delete job"},

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
	de := detailsError{code, ""}
	if internalErr != nil {
		de.details = internalErr.Error()
	}

	he := echo.NewHTTPError(find(code).httpStatus, de)
	if internalErr != nil {
		he.Internal = internalErr
	}
	return he
}

type detailsError struct {
	errorCode ServiceErrorCode
	details   interface{}
}

// instead of sending a ServiceErrorCode as he.Message, send the validation error string (see above)
func HTTPErrorWithDetails(code ServiceErrorCode, internalErr error, details string) error {
	se := find(code)
	he := echo.NewHTTPError(se.httpStatus, detailsError{code, details})
	if internalErr != nil {
		he.Internal = internalErr
	}
	return he
}

// Convert a ServiceErrorCode into an Error as defined in openapi.v2.yml
// serviceError is optional, prevents multiple find() calls
func APIError(serviceError *serviceError, c echo.Context, details interface{}) *Error {
	operationID, ok := c.Get("operationID").(string)
	if !ok || operationID == "" {
		serviceError = find(ErrorMalformedOperationID)
	}

	apiErr := &Error{
		Href:        fmt.Sprintf("%s/%d", ErrorHREF, serviceError.code),
		Id:          fmt.Sprintf("%d", serviceError.code),
		Kind:        "Error",
		Code:        fmt.Sprintf("%s%d", ErrorCodePrefix, serviceError.code),
		OperationId: operationID, // set operation id from context
		Reason:      serviceError.reason,
	}
	if details != nil {
		apiErr.Details = &details
	}
	return apiErr
}

// Helper to make the ErrorList as defined in openapi.v2.yml
func APIErrorList(page int, pageSize int, c echo.Context) *ErrorList {
	list := &ErrorList{
		Kind:  "ErrorList",
		Page:  page,
		Size:  0,
		Total: len(getServiceErrors()),
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
		// Implicit memory alasing doesn't couse any bug in this case
		/* #nosec G601 */
		list.Items = append(list.Items, *APIError(&e, c, nil))
	}
	list.Size = len(list.Items)
	return list
}

func apiErrorFromEchoError(echoError *echo.HTTPError) ServiceErrorCode {
	switch echoError.Code {
	case http.StatusNotFound:
		return ErrorResourceNotFound
	case http.StatusBadRequest:
		return ErrorBadRequest
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
	doResponse := func(details interface{}, code ServiceErrorCode, c echo.Context, internal error) {
		if !c.Response().Committed {
			var err error
			se := find(code)
			apiErr := APIError(se, c, details)

			if se.httpStatus == http.StatusInternalServerError {
				errMsg := fmt.Sprintf("Internal server error. Code: %s, OperationId: %s", apiErr.Code, apiErr.OperationId)

				if internal != nil {
					errMsg += fmt.Sprintf(", InternalError: %v", internal)
				}

				c.Logger().Error(errMsg)
			}

			if c.Request().Method == http.MethodHead {
				err = c.NoContent(se.httpStatus)
			} else {
				err = c.JSON(se.httpStatus, apiErr)
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
		doResponse(echoError.Error(), ErrorNotHTTPError, c, echoError)
		return
	}

	err, ok := he.Message.(detailsError)
	if !ok {
		// No service code was set, so Echo or the generated code threw this error
		doResponse(he.Error(), apiErrorFromEchoError(he), c, he.Internal)
		return
	}
	doResponse(err.details, err.errorCode, c, he.Internal)
}
