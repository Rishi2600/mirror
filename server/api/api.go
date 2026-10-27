// Package api holds the API contract, the code generated from it, and the
// embedded copy of the contract that the server serves.
package api

import _ "embed"

//go:generate go tool oapi-codegen -config oapi-codegen.yaml -o api.gen.go openapi.yaml

// Spec is the OpenAPI contract, embedded so the binary runs from any folder.
//
//go:embed openapi.yaml
var Spec []byte
