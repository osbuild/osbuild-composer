package container

import (
	"context"
	"fmt"
	"strings"
)

type resolveResult struct {
	spec Spec
	err  error
}

type Resolver struct {
	jobs  int
	queue chan resolveResult

	ctx context.Context

	Arch string
}

func NewResolver(arch string) Resolver {
	return Resolver{
		ctx:   context.Background(),
		queue: make(chan resolveResult, 2),
		Arch:  arch,
	}
}

func (r *Resolver) Add(source, name string, TLSVerify *bool) {
	client, err := NewClient(source)
	r.jobs += 1

	if err != nil {
		r.queue <- resolveResult{err: err}
		return
	}

	client.SetTLSVerify(TLSVerify)
	client.SetArchitectureChoice(r.Arch)

	go func() {
		spec, err := client.Resolve(r.ctx, name)
		if err != nil {
			err = fmt.Errorf("'%s': %w", source, err)
		}
		r.queue <- resolveResult{spec: spec, err: err}
	}()
}

func (r *Resolver) Finish() ([]Spec, error) {

	specs := make([]Spec, 0, r.jobs)
	errs := make([]string, 0, r.jobs)
	for r.jobs > 0 {
		result := <-r.queue
		r.jobs -= 1

		if result.err == nil {
			specs = append(specs, result.spec)
		} else {
			errs = append(errs, result.err.Error())
		}
	}

	if len(errs) > 0 {
		detail := strings.Join(errs, "; ")
		return specs, fmt.Errorf("failed to resolve container: %s", detail)
	}

	return specs, nil
}
