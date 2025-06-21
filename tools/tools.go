// Do not remove the following build tag line: It exempts this file from normal
// builds, which would fail because the imports are programs – package main –
// and not really importable packages.
//
//go:build tools

// Package tools provides build tools necessary for go-eigentrust.
package tools

// Put only installable tools into this list.
// scripts/install_build_tools.sh parses these imports to install them.

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/ogen-go/ogen/cmd/ogen"
	_ "github.com/yoheimuta/protolint/cmd/protolint"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
