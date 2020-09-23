//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen -package=api -generate types,server -o api.gen.go openapi.yml

package api

const BasePath = "/api/composer-koji/v1"
