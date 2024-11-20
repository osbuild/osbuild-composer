package main

var (
	WorkerClientErrorFrom         = workerClientErrorFrom
	MakeJobErrorFromOsbuildOutput = makeJobErrorFromOsbuildOutput
	Main                          = main
)

func MockRun(new func()) (restore func()) {
	saved := run
	run = new
	return func() {
		run = saved
	}
}
