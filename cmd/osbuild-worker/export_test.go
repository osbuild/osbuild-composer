package main

var (
	WorkerClientErrorFrom         = workerClientErrorFrom
	MakeJobErrorFromOsbuildOutput = makeJobErrorFromOsbuildOutput
	Main                          = main
	ParseManifestPipelines        = parseManifestPipelines
)

func MockRun(new func()) (restore func()) {
	saved := run
	run = new
	return func() {
		run = saved
	}
}

type ResolveBootcInfoFuncType = resolveBootcInfoFuncType

func MockResolveBootcInfoFunc(mockFunc ResolveBootcInfoFuncType) (restore func()) {
	saved := resolveBootcInfoFunc
	resolveBootcInfoFunc = mockFunc
	return func() {
		resolveBootcInfoFunc = saved
	}
}

func MockResolveBootcBuildInfoFunc(mockFunc ResolveBootcInfoFuncType) (restore func()) {
	saved := resolveBootcBuildInfoFunc
	resolveBootcBuildInfoFunc = mockFunc
	return func() {
		resolveBootcBuildInfoFunc = saved
	}
}
