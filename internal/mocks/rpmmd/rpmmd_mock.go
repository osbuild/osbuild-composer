package rpmmd_mock

import (
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type Fixture struct {
	*store.Store
	Workers *worker.Server
	dnfjson_mock.ResponseGenerator
}
