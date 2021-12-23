package clienterrors

const (
	ErrorNoDynamicArgs       ClientErrorCode = 1
	ErrorInvalidTargetConfig ClientErrorCode = 2
	ErrorSharingTarget       ClientErrorCode = 3
	ErrorInvalidTarget       ClientErrorCode = 4

	ErrorDNFError             ClientErrorCode = 1001
	ErrorDepsolveJob          ClientErrorCode = 1002
	ErrorDepsolveDependency   ClientErrorCode = 1003
	ErrorParsingDynamicArgs   ClientErrorCode = 1004
	ErrorManifestGeneration   ClientErrorCode = 1005
	ErrorManifestDependency   ClientErrorCode = 1006
	ErrorBuildJob             ClientErrorCode = 1007
	ErrorUploadingImage       ClientErrorCode = 1008
	ErrorRegisteringImage     ClientErrorCode = 1009
	ErrorKojiFailedDependency ClientErrorCode = 10010
	ErrorKojiBuild            ClientErrorCode = 1011
	ErrorKojiInit             ClientErrorCode = 1012
	ErrorKojiFinalize         ClientErrorCode = 1013
	ErrorInvalidConfig        ClientErrorCode = 1014
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
