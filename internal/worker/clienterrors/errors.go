package clienterrors

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

	ErrorDNFDepsolveError ClientErrorCode = 20
	ErrorDNFMarkingErrors ClientErrorCode = 21
	ErrorDNFOtherError    ClientErrorCode = 22
	ErrorRPMMDError       ClientErrorCode = 23
)

type ClientErrorCode int

type Error struct {
	ID      ClientErrorCode `json:"id"`
	Reason  string          `json:"reason"`
	Details interface{}     `json:"details"`
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
	case ErrorNoDynamicArgs:
		return JobStatusUserInputError
	case ErrorInvalidTargetConfig:
		return JobStatusUserInputError
	case ErrorSharingTarget:
		return JobStatusUserInputError
	case ErrorInvalidTarget:
		return JobStatusUserInputError
	case ErrorEmptyManifest:
		return JobStatusInternalError
	default:
		return JobStatusInternalError
	}
}

func WorkerClientError(code ClientErrorCode, reason string, details ...interface{}) *Error {
	return &Error{
		ID:      code,
		Reason:  reason,
		Details: details,
	}
}
