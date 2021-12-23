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

	ErrorDNFDepsolveError ClientErrorCode = 20
	ErrorDNFMarkingError  ClientErrorCode = 21
	ErrorDNFOtherError    ClientErrorCode = 22
	ErrorRPMMDError       ClientErrorCode = 23
)

type ClientErrorCode int

type Error struct {
	ID      ClientErrorCode `json:"id"`
	Reason  string          `json:"reason"`
	Details interface{}     `json:"details"`
}

func WorkerClientError(code ClientErrorCode, reason string, details ...interface{}) *Error {
	return &Error{
		ID:      code,
		Reason:  reason,
		Details: details,
	}
}
