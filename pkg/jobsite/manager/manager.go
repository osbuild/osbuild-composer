package manager

type ArgumentList []string

func (AL *ArgumentList) String() string {
	return ""
}

func (AL *ArgumentList) Set(value string) error {
	*AL = append(*AL, value)
	return nil
}

type BuildRequest struct {
	Pipelines    []string `json:"pipelines"`
	Environments []string `json:"environments"`
}

type Step func(chan<- struct{}, chan<- error)
