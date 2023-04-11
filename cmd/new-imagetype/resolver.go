package main

import "fmt"

type resource interface {
	isResource()
}

type resolver interface {
	Type() string
	Add(spec resource)
}

type resolvers map[string]resolver

func (r resolvers) Register(slv resolver) {
	if _, found := r[slv.Type()]; found {
		panic(fmt.Sprintf("resolver for %q already registered", slv.Type()))
	}
	r[slv.Type()] = slv
}

type FileResolver struct {
	resType string
	specs   []URL
}

func NewFileResolver() *FileResolver {
	return &FileResolver{
		resType: "org.osbuild.remotefile",
	}
}

func (fr FileResolver) Type() string {
	return fr.resType
}

type URL string

func (URL) isResource() {}

func (fr *FileResolver) Add(spec resource) {
	url, ok := spec.(URL)
	if !ok {
		panic(fmt.Sprintf("wrong resource type %T for resolver %T", spec, fr))
	}
	fr.specs = append(fr.specs, url)
}
