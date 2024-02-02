//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate /bin/bash scripts/generate-manifest
//go:generate go run ./pkg/bindata

package main
