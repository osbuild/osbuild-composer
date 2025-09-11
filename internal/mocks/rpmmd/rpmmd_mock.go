package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type Fixture struct {
	StoreFixture *store.Fixture
	Workers      *worker.Server
}
