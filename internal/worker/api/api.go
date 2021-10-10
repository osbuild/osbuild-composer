//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen -package=api -generate types,server,spec -o api.gen.go openapi.yml

package api

// default basepath, can be overwritten
var BasePath = "/api/worker/v1"
