package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type FixtureGenerator func(tmpdir string) Fixture

func createBaseWorkersFixture(tmpdir string) *worker.Server {
	q, err := fsjobqueue.New(tmpdir)
	if err != nil {
		panic(err)
	}
	return worker.NewServer(nil, q, worker.Config{BasePath: "/api/worker/v1"})
}

func BaseFixture(tmpdir string) Fixture {
	return Fixture{
		store.FixtureBase(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}

func NoComposesFixture(tmpdir string) Fixture {
	return Fixture{
		store.FixtureEmpty(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}

func NonExistingPackage(tmpdir string) Fixture {
	return Fixture{
		store.FixtureBase(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.NonExistingPackage,
	}
}

func BadDepsolve(tmpdir string) Fixture {
	return Fixture{
		store.FixtureBase(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.BadDepsolve,
	}
}

func BadFetch(tmpdir string) Fixture {
	return Fixture{
		store.FixtureBase(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.BadFetch,
	}
}

func OldChangesFixture(tmpdir string) Fixture {
	return Fixture{
		store.FixtureOldChanges(),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}
