package clienterrors

import (
	"fmt"
)

const (
	ErrorNoDynamicArgs        ClientErrorCode = 1
	ErrorInvalidTargetConfig  ClientErrorCode = 2
	ErrorSharingTarget        ClientErrorCode = 3
	ErrorInvalidTarget        ClientErrorCode = 4
	ErrorDepsolveDependency   ClientErrorCode = 5
	ErrorReadingJobStatus     ClientErrorCode = 6
	ErrorParsingDynamicArgs   ClientErrorCode = 7
	ErrorManifestGeneration   ClientErrorCode = 8
	ErrorManifestDependency   ClientErrorCode = 9
	ErrorBuildJob             ClientErrorCode = 10
	ErrorUploadingImage       ClientErrorCode = 11
	ErrorImportingImage       ClientErrorCode = 12
	ErrorKojiFailedDependency ClientErrorCode = 13
	ErrorKojiBuild            ClientErrorCode = 14
	ErrorKojiInit             ClientErrorCode = 15
	ErrorKojiFinalize         ClientErrorCode = 16
	ErrorInvalidConfig        ClientErrorCode = 17
	ErrorOldResultCompatible  ClientErrorCode = 18
	ErrorEmptyManifest        ClientErrorCode = 19
	ErrorDNFDepsolveError     ClientErrorCode = 20
	ErrorDNFMarkingErrors     ClientErrorCode = 21
	ErrorDNFOtherError        ClientErrorCode = 22
	ErrorRPMMDError           ClientErrorCode = 23
	ErrorEmptyPackageSpecs    ClientErrorCode = 24
	ErrorDNFRepoError         ClientErrorCode = 25
	ErrorJobDependency        ClientErrorCode = 26
	ErrorJobMissingHeartbeat  ClientErrorCode = 27
	ErrorTargetError          ClientErrorCode = 28
	ErrorParsingJobArgs       ClientErrorCode = 29
	ErrorContainerResolution  ClientErrorCode = 30
	ErrorContainerDependency  ClientErrorCode = 31
	ErrorOSTreeRefInvalid     ClientErrorCode = 32
	ErrorOSTreeRefResolution  ClientErrorCode = 33
	ErrorOSTreeParamsInvalid  ClientErrorCode = 34
	ErrorOSTreeDependency     ClientErrorCode = 35
	ErrorRemoteFileResolution ClientErrorCode = 36
	ErrorJobPanicked          ClientErrorCode = 37
	ErrorGeneratingSignedURL  ClientErrorCode = 38
	ErrorInvalidRepositoryURL ClientErrorCode = 39
)

type ClientErrorCode int

type Error struct {
	ID      ClientErrorCode `json:"id"`
	Reason  string          `json:"reason"`
	Details interface{}     `json:"details,omitempty"`
}

func (e *Error) String() string {
	return fmt.Sprintf("Code: %d, Reason: %s, Details: %v", e.ID, e.Reason, e.Details)
}

const (
	JobStatusSuccess        = "2xx"
	JobStatusUserInputError = "4xx"
	JobStatusInternalError  = "5xx"
)

type StatusCode string

func (s *StatusCode) ToString() string {
	return string(*s)
}

func GetStatusCode(err *Error) StatusCode {
	if err == nil {
		return JobStatusSuccess
	}
	switch err.ID {
	case ErrorDNFDepsolveError:
		return JobStatusUserInputError
	case ErrorDNFMarkingErrors:
		return JobStatusUserInputError
	case ErrorDNFRepoError:
		return JobStatusInternalError
	case ErrorNoDynamicArgs:
		return JobStatusUserInputError
	case ErrorInvalidTargetConfig:
		return JobStatusUserInputError
	case ErrorSharingTarget:
		return JobStatusUserInputError
	case ErrorInvalidTarget:
		return JobStatusUserInputError
	case ErrorTargetError:
		return JobStatusUserInputError
	case ErrorDepsolveDependency:
		return JobStatusUserInputError
	case ErrorManifestDependency:
		return JobStatusUserInputError
	case ErrorJobDependency:
		return JobStatusUserInputError
	case ErrorEmptyPackageSpecs:
		return JobStatusUserInputError
	case ErrorEmptyManifest:
		return JobStatusUserInputError
	case ErrorContainerResolution:
		return JobStatusUserInputError
	case ErrorOSTreeDependency:
		return JobStatusUserInputError
	default:
		return JobStatusInternalError
	}
}

// IsDependencyError returns true if the error means that a dependency of a job failed
func (e *Error) IsDependencyError() bool {
	switch e.ID {
	case ErrorContainerDependency:
		return true
	case ErrorOSTreeDependency:
		return true
	case ErrorDepsolveDependency:
		return true
	case ErrorManifestDependency:
		return true
	case ErrorKojiFailedDependency:
		return true
	case ErrorJobDependency:
		return true
	default:
		return false
	}
}

func New(code ClientErrorCode, reason string, details interface{}) *Error {
	return &Error{
		ID:      code,
		Reason:  reason,
		Details: details,
	}
}
