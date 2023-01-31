package remotefile

import "github.com/osbuild/osbuild-composer/internal/worker/clienterrors"

type Spec struct {
	URL             string
	Content         []byte
	ResolutionError *clienterrors.Error
}
