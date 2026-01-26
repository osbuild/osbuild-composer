package container

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type resolveResult struct {
	spec Spec
	err  error
}

type Resolver interface {
	Add(spec SourceSpec)
	Finish() ([]Spec, error)

	Resolve(spec SourceSpec) (Spec, error)
	ResolveAll(sources map[string][]SourceSpec) (map[string][]Spec, error)
}

type asyncResolver struct {
	jobs  int
	queue chan resolveResult

	Arch         string
	AuthFilePath string

	newClient func(string) (*Client, error)
}

type SourceSpec struct {
	Source    string
	Name      string
	Digest    *string
	TLSVerify *bool
	Local     bool
}

// XXX: use arch.Arch here?
func NewResolver(arch string) *asyncResolver {
	// NOTE: this should return the Resolver interface, but osbuild-composer
	// sets the AuthFilePath and for now we don't want to break the API.
	return &asyncResolver{
		queue: make(chan resolveResult, 2),
		Arch:  arch,

		newClient: NewClient,
	}
}

func (r *asyncResolver) Add(spec SourceSpec) {
	client, err := r.newClient(spec.Source)
	r.jobs += 1

	if err != nil {
		r.queue <- resolveResult{err: err}
		return
	}

	client.SetTLSVerify(spec.TLSVerify)
	client.SetArchitectureChoice(r.Arch)
	if r.AuthFilePath != "" {
		client.SetAuthFilePath(r.AuthFilePath)
	}

	go func() {
		ctx, cancelTimeout := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelTimeout()
		spec, err := client.Resolve(ctx, spec.Name, spec.Local)
		if err != nil {
			err = fmt.Errorf("'%s': %w", spec.Source, err)
		}
		r.queue <- resolveResult{spec: spec, err: err}
	}()
}

func (r *asyncResolver) Finish() ([]Spec, error) {
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

	// Return a stable result, sorted by Digest
	sort.Slice(specs, func(i, j int) bool { return specs[i].Digest < specs[j].Digest })

	return specs, nil
}

func (r *asyncResolver) Resolve(spec SourceSpec) (Spec, error) {
	r.Add(spec)
	resSlice, err := r.Finish()
	if err != nil {
		return Spec{}, err
	}

	if len(resSlice) != 1 {
		return Spec{}, fmt.Errorf("unexpected results in container resolver: expected 1 but received %d", len(resSlice))
	}

	return resSlice[0], nil
}

func (r *asyncResolver) ResolveAll(sources map[string][]SourceSpec) (map[string][]Spec, error) {
	containers := make(map[string][]Spec, len(sources))
	for name, srcList := range sources {
		containerSpecs := make([]Spec, len(srcList))
		for idx, src := range srcList {
			res, err := r.Resolve(src)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve container: %w", err)
			}
			containerSpecs[idx] = res
		}
		containers[name] = containerSpecs
	}

	return containers, nil
}

type blockingResolver struct {
	Arch         string
	AuthFilePath string

	newClient func(string) (*Client, error)

	results []resolveResult
}

// NewBlockingResolver returns a [asyncResolver] that resolves container refs
// synchronously (blocking).
// TODO: Make this the only resolver after all clients have migrated to this.
func NewBlockingResolver(arch string) Resolver {
	return &blockingResolver{
		Arch:      arch,
		newClient: NewClient,
	}
}

func (r *blockingResolver) Add(src SourceSpec) {
	spec, err := r.Resolve(src)
	if err != nil {
		err = fmt.Errorf("'%s': %w", src.Source, err)
	}
	r.results = append(r.results, resolveResult{spec: spec, err: err})
}

func (r *blockingResolver) Finish() ([]Spec, error) {
	specs := make([]Spec, 0, len(r.results))
	errs := make([]string, 0, len(r.results))
	for _, result := range r.results {
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

	// Return a stable result, sorted by Digest
	sort.Slice(specs, func(i, j int) bool { return specs[i].Digest < specs[j].Digest })

	return specs, nil
}

func (r *blockingResolver) Resolve(source SourceSpec) (Spec, error) {
	client, err := r.newClient(source.Source)
	if err != nil {
		return Spec{}, err
	}

	client.SetTLSVerify(source.TLSVerify)
	client.SetArchitectureChoice(r.Arch)
	if r.AuthFilePath != "" {
		client.SetAuthFilePath(r.AuthFilePath)
	}

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelTimeout()
	return client.Resolve(ctx, source.Name, source.Local)
}

// ResolveAll calls [Resolve] with each container source spec in the map and
// returns a map of results with the corresponding keys as the input argument.
func (r *blockingResolver) ResolveAll(sources map[string][]SourceSpec) (map[string][]Spec, error) {
	containers := make(map[string][]Spec, len(sources))
	for name, srcList := range sources {
		containerSpecs := make([]Spec, len(srcList))
		for idx, src := range srcList {
			res, err := r.Resolve(src)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve container: %w", err)
			}
			containerSpecs[idx] = res
		}
		containers[name] = containerSpecs
	}

	return containers, nil
}
