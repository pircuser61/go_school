// nolint
//
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.9.1 -package api -generate types,chi-server -o ../internal/api/api.go api.swagger.yaml
package api
