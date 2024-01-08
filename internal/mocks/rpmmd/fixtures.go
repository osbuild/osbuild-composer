package rpmmd_mock

import (
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type FixtureGenerator func(tmpdir, hostDistroName, hostArchName string) Fixture

func createBaseWorkersFixture(tmpdir string) *worker.Server {
	q, err := fsjobqueue.New(tmpdir)
	if err != nil {
		panic(err)
	}
	return worker.NewServer(nil, q, worker.Config{BasePath: "/api/worker/v1"})
}

func BaseFixture(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureBase(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}

func NoComposesFixture(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureEmpty(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}

func NonExistingPackage(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureBase(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.NonExistingPackage,
	}
}

func BadDepsolve(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureBase(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.BadDepsolve,
	}
}

func BadFetch(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureBase(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.BadFetch,
	}
}

func OldChangesFixture(tmpdir, hostDistroName, hostArchName string) Fixture {
	return Fixture{
		store.FixtureOldChanges(hostDistroName, hostArchName),
		createBaseWorkersFixture(tmpdir),
		dnfjson_mock.Base,
	}
}

func BadJobJSONFixture(tmpdir, hostDistroName, hostArchName string) Fixture {
	err := os.Mkdir(path.Join(tmpdir, "/jobs"), 0755)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(path.Join(tmpdir, "/jobs/30000000-0000-0000-0000-000000000005.json"), []byte("{invalid json content"), 0600)
	if err != nil {
		panic(err)
	}

	return Fixture{
		store.FixtureJobs(hostDistroName, hostArchName),
		createBaseWorkersFixture(path.Join(tmpdir, "/jobs")),
		dnfjson_mock.Base,
	}
}
