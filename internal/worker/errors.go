package worker

type ResultCode int

const (
	DNFError           ResultCode = 0
	DepsolveError      ResultCode = 1
	ManifestByIDError  ResultCode = 2
	OsbuildError       ResultCode = 3
	KojiInitError      ResultCode = 4
	KojiFinializeError ResultCode = 5
	OsbuildKojiError   ResultCode = 6
	ImageUploadError   ResultCode = 7
	ImageShareError    ResultCode = 8
	JobSuccess         ResultCode = 9
	UnspecifiedError   ResultCode = 1000
)

type WorkerError struct {
	Message string
	Code    ResultCode
}

func (err *WorkerError) Error() string {
	return err.Message
}

type ShareError WorkerError

func NewShareError(err error) ShareError {
	return ShareError{
		Message: err.Error(),
		Code:    ImageShareError,
	}
}

func (err *ShareError) Error() string {
	return err.Message
}

type UploadError WorkerError

func NewUploadError(err error) UploadError {
	return UploadError{
		Message: err.Error(),
		Code:    ImageUploadError,
	}
}

func (err *UploadError) Error() string {
	return err.Message
}
