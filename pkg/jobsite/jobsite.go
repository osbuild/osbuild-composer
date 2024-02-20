package jobsite

const (
	ExitOk int = iota
	ExitError
	ExitTimeout
	ExitSignal
)

type BuildRequest struct {
	Pipelines    []string `json:"pipelines"`
	Environments []string `json:"environments"`
}
