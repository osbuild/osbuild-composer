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
