package jobqueue

import "io"

type LocalTargetUploader interface {
	UploadImage(job *Job, reader io.Reader) error
}
