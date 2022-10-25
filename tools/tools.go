// Do not remove the following build tag line: It exempts this file from normal
// builds, which would fail because the imports are programs – package main –
// and not really importable packages.
//
//go:build tools

// Package tools provides build tools necessary for go-eigentrust.
package tools

// Put only installable tools into this list.
// scripts/install_build_tools.sh parses these imports to install them.
