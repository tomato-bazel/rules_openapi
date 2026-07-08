//go:build tools
// +build tools

// Package main keeps build-time-only module dependencies that no source file
// imports directly, so `go mod tidy` retains them in go.mod / go.sum. The
// generated Go clients that openapi_go_client produces import
// github.com/oapi-codegen/runtime; the go_deps extension must be able to
// resolve it from this module's lockfile.
package main

import _ "github.com/oapi-codegen/runtime"
