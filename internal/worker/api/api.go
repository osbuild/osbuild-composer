//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package=api -generate types,server,spec -o api.gen.go openapi.yml

package api

// default basepath, can be overwritten
var BasePath = "/api/worker/v1"
