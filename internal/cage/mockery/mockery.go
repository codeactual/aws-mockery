// Copyright (C) 2019 The CodeActual Go Environment Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package mockery provides a limited-feature runner that uses the vendored instance
// instead of executing a process.
package mockery

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/vektra/mockery/mockery"

	cage_mod "github.com/codeactual/aws-mockery/internal/cage/go/mod"
)

// Config defines the limited set of options, usually capture from the console,
// that are used in this command.
type Config struct {
	InPackage bool
	Name      string
	Dir       string
}

// Run is a substitute for the full CLI that uses the vendored version and only
// the limited set of required options. It is an abbreviated cmd/mockery/mockery.go.
func Run(c Config) (string, error) {
	var buf bytes.Buffer

	stream := &StreamProvider{W: &buf}
	visitor := &mockery.GeneratorVisitor{
		InPackage:   c.InPackage,
		Osp:         stream,
		PackageName: "mocks",
	}
	walker := mockery.Walker{
		BaseDir:  c.Dir,
		Filter:   regexp.MustCompile(fmt.Sprintf("^%s$", c.Name)),
		LimitOne: true,
	}

	// Force mockery dependencies such as go/build and golang.org/x/tools/go/packages to use module mode when
	// querying packages by file path instead of import path. It avoids this type of error:
	// `-: import "/path/to/dir/in/query": cannot import absolute path`.
	if origGo111 := os.Getenv("GO111MODULE"); origGo111 != "on" {
		if updateErr := os.Setenv("GO111MODULE", "on"); updateErr != nil {
			return "", errors.Wrap(updateErr, "failed to set GO111MODULE=on")
		}
		defer func() {
			if restoreErr := os.Setenv("GO111MODULE", "on"); restoreErr != nil {
				fmt.Fprintf(os.Stderr, "failed to restore GO111MODULE value to [%s]: %+v\n", origGo111, restoreErr)
			}
		}()
	}

	// Support vendored SDKs.
	if cage_mod.IsPossibleVendorPath(c.Dir) {
		if origGoflags := os.Getenv("GOFLAGS"); !strings.Contains(origGoflags, "-mod=vendor") {
			newGoflags := origGoflags
			if newGoflags != "" {
				newGoflags += " "
			}
			newGoflags += "-mod=vendor"
			if updateErr := os.Setenv("GOFLAGS", newGoflags); updateErr != nil {
				return "", errors.Wrap(updateErr, "failed to enable vendoring in GOFLAGS")
			}
			defer func() {
				if restoreErr := os.Setenv("GOFLAGS", "on"); restoreErr != nil {
					fmt.Fprintf(os.Stderr, "failed to restore GOFLAGS value to [%s]: %+v\n", origGoflags, restoreErr)
				}
			}()
		}
	}

	generated := walker.Walk(visitor)

	if c.Name != "" && !generated {
		return "", errors.Errorf("Unable to find %s in any go files under %s", c.Name, c.Dir)
	}
	return buf.String(), nil
}

// StreamProvider is just mockery.StdoutStreamProvider with a variable writer.
type StreamProvider struct {
	W io.Writer
}

// GetWriter returns the configured writer.
func (s *StreamProvider) GetWriter(iface *mockery.Interface) (io.Writer, error, mockery.Cleanup) {
	return s.W, nil, func() error { return nil }
}
