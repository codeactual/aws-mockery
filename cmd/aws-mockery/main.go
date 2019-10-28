// Copyright (C) 2019 The aws-mockery Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command aws-mockery uses the mockery API to generate implementations of selected aws-sdk-go service interfaces.
//
// Currently only aws-sdk-go v1 is supported.
//
// Usage:
//
//   aws-mockery --help
//
// Output mock implementations for KMS, Route53, and SNS:
//
//   aws-mockery --out-dir /path/to/mocks --sdk-dir /path/to/github.com/aws/aws-sdk-go --service=kms,route53,sns
package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/codeactual/aws-mockery/internal/cage/cli/handler"
	handler_cobra "github.com/codeactual/aws-mockery/internal/cage/cli/handler/cobra"
	log_zap "github.com/codeactual/aws-mockery/internal/cage/cli/handler/mixin/log/zap"
	cage_mockery "github.com/codeactual/aws-mockery/internal/cage/mockery"
	cage_file "github.com/codeactual/aws-mockery/internal/cage/os/file"
	cage_filepath "github.com/codeactual/aws-mockery/internal/cage/path/filepath"
	cage_reflect "github.com/codeactual/aws-mockery/internal/cage/reflect"
	cage_strings "github.com/codeactual/aws-mockery/internal/cage/strings"
)

const (
	newFilePerm = 0666 // Match os.Create for use with ioutil.WriteFile
	newDirPerm  = 0755
)

// serviceName holds purpose-specific AWS service names collected from the aws-sdk-go
// repository. Each type of name, per service, supports code generation of compatible
// clients and a client provider.
type serviceName struct {
	// Interface is used for generating return types (example: "ec2iface.EC2API").
	Interface string
	// InterfaceShort is used for generating type assertions (example: "EC2API").
	InterfaceShort string
	// InterfacePackage is used for generating mock clients (example: "ec2iface").
	InterfacePackage string
	// Package is used for generating package function invocations (example: "ec2").
	Package string
	// Proper is used for generating provider method names (example: "EC2").
	Proper string
}

func main() {
	err := handler_cobra.NewHandler(&Handler{}).Execute()
	if err != nil {
		panic(errors.WithStack(err))
	}
}

// Handler defines the sub-command flags and logic.
type Handler struct {
	handler.IO

	OutDir   string `usage:""`
	SdkDir   string `usage:""`
	SdkVer   string `usage:"1 or 2"`
	Services string `usage:"Comma-separated list of service IDs (SDK dir names under service/)"`

	Log *log_zap.Mixin

	serviceDir string
}

// Init defines the command, its environment variable prefix, etc.
//
// It implements cli/handler/cobra.Handler.
func (h *Handler) Init() handler_cobra.Init {
	h.Log = &log_zap.Mixin{}

	return handler_cobra.Init{
		Cmd: &cobra.Command{
			Use:   "aws-mockery",
			Short: "Generate mocks of selected aws-sdk-go (v1/v2) services",
		},
		EnvPrefix: "AWS_MOCKERY",
		Mixins: []handler.Mixin{
			h.Log,
		},
	}
}

// BindFlags binds the flags to Handler fields.
//
// It implements cli/handler/cobra.Handler.
func (h *Handler) BindFlags(cmd *cobra.Command) []string {
	cmd.Flags().StringVarP(&h.OutDir, "out-dir", "", "", cage_reflect.GetFieldTag(*h, "OutDir", "usage"))
	cmd.Flags().StringVarP(&h.SdkDir, "sdk-dir", "", "", cage_reflect.GetFieldTag(*h, "SdkDir", "usage"))
	cmd.Flags().StringVarP(&h.SdkVer, "sdk-ver", "", "1", cage_reflect.GetFieldTag(*h, "SdkVer", "usage"))
	cmd.Flags().StringVarP(&h.Services, "service", "", "", cage_reflect.GetFieldTag(*h, "Services", "usage"))
	return []string{"out-dir", "sdk-dir", "service"}
}

// Run performs the sub-command logic.
//
// It implements cli/handler/cobra.Handler.
func (h *Handler) Run(ctx context.Context, args []string) {
	h.serviceDir = filepath.Join(h.SdkDir, "service")

	// Improve 1.11+ modules support consistence with predictable working directories and absolute paths.
	//
	// Working directories
	//
	// For example, in x/tools/go/packages query results can depend on both the working directory
	// (e.g. if it's in a module/GOPATH or not, etc.) and the file/dir/import-path queried.
	// Here we remove the former variable from permutations because the working directory should not effect
	// the files generated.
	//
	// **Absolute paths**
	//
	// Prevent x/tools/go/packages queries for file/dir paths from being proessed as as import paths.
	//
	// **Non-module, non-vendor, non-GOPATH --sdk-dir locations**
	//
	// If the "aws-sdk-go/go.mod" does not exist, temporarily create it as a hack for the Go toolchain
	// to process it as a module.
	h.Log.ExitOnErr(1, cage_filepath.Abs(&h.serviceDir))
	h.Log.ExitOnErr(1, cage_filepath.Abs(&h.OutDir))
	exists, _, existsErr := cage_file.Exists(h.SdkDir)
	h.Log.ExitOnErr(1, existsErr)
	if !exists {
		h.Exitf(1, "--service selection [%s] not found", h.SdkDir)
	}
	tmpGomodPath := filepath.Join(h.SdkDir, "go.mod")
	exists, _, existsErr = cage_file.Exists(tmpGomodPath)
	h.Log.ExitOnErr(1, existsErr)
	if !exists {
		gomodFile, createErr := os.Create(tmpGomodPath)
		h.Log.ExitOnErr(1, createErr)

		defer func() {
			if rmErr := cage_file.RemoveSafer(tmpGomodPath); rmErr != nil {
				fmt.Fprintf(h.Err(), "failed to remove [%s]: %+v", tmpGomodPath, rmErr)
			}
		}()

		_, writeErr := gomodFile.WriteString("module github.com/aws/aws-sdk-go\n")
		h.Log.ExitOnErr(1, writeErr)
	}

	svcIds := cage_strings.Split(h.Services, ",")
	if len(svcIds) == 0 {
		h.Exit(1, "no --service selected")
	}

	availServices, err := h.getAvailServices()
	h.Log.ExitOnErr(1, err)

	var services []serviceName
	for _, svcId := range svcIds {
		svc, found := availServices[svcId]
		if !found {
			h.Exitf(1, "-service selection [%s] not found in SDK", svcId)
		}
		services = append(services, svc)
	}

	h.Log.ExitOnErr(1, h.addMockClients(services))
}

func (h *Handler) addMockClients(services []serviceName) (err error) {
	var lastSvcPkg string

	for _, svc := range services {
		lastSvcPkg = svc.Package

		ifaceDir := filepath.Join(h.serviceDir, svc.Package, svc.InterfacePackage)
		outFileName := filepath.Join(h.OutDir, svc.Package+".go")
		outPkgName := filepath.Base(h.OutDir)

		if err = h.addMockClient(svc, ifaceDir, outFileName, outPkgName); err != nil {
			break
		}
	}

	if err != nil {
		return errors.Wrapf(err, "failed to add mock client for [%s]", lastSvcPkg)
	}

	return nil
}

func (h *Handler) addMockClient(svc serviceName, ifaceDir, outFileName, outPkgName string) error {
	// Improve 1.11+ modules support consistence with predictable working directories and absolute paths.
	//
	// **Working directories**
	//
	// For example, in x/tools/go/packages query results can depend on both the working directory
	// (e.g. if it's in a module/GOPATH or not, etc.) and the file/dir/import-path queried.
	// Here we remove the former variable from permutations because the working directory should not effect
	// the files generated.
	//
	// Absolute paths
	//
	// Prevent x/tools/go/packages queries for file/dir paths from being proessed as as import paths.
	//
	// Non-module, non-vendor, non-GOPATH --sdk-dir locations
	//
	// If the "aws-sdk-go/go.mod" does not exist, temporarily create it as a hack for the Go toolchain
	// to process it as a module.
	if err := os.Chdir(ifaceDir); err != nil {
		return errors.Wrapf(err, "failed to change working directory to [%s]", ifaceDir)
	}

	outStr, err := cage_mockery.Run(cage_mockery.Config{
		Dir:  ifaceDir,
		Name: svc.InterfaceShort,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to generate [%s] with mockery, output [%s]", outFileName, outStr)
	}

	outDirName := filepath.Dir(outFileName)

	// perform final adjustments to the source
	newPkgAndImport := fmt.Sprintf(
		"package %s\n\n"+
			"import \"github.com/aws/aws-sdk-go/service/%s/%s\"",
		outPkgName,
		svc.Package,
		svc.InterfacePackage,
	)
	outStr = strings.Replace(outStr, "package mocks", newPkgAndImport, 1)
	outStr += fmt.Sprintf("\n\nvar _ %s = (*%s)(nil)\n", svc.Interface, svc.InterfaceShort)

	formatted, err := format.Source([]byte(outStr))
	if err != nil {
		return errors.Wrapf(err, "failed to format mock client [%s]", outFileName)
	}

	err = os.MkdirAll(outDirName, newDirPerm)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir [%s]", outDirName)
	}

	if err = ioutil.WriteFile(outFileName, formatted, newFilePerm); err != nil {
		return errors.Wrapf(err, "failed to write mock client to [%s]", outFileName)
	}

	return nil
}

// getAvailServices returns a ServiceName for each service found in the SDK.
func (h *Handler) getAvailServices() (map[string]serviceName, error) {
	avail := make(map[string]serviceName)

	mocksDir, err := os.Open(h.serviceDir) // #nosec G304
	if err != nil {
		return map[string]serviceName{}, errors.Wrap(err, "failed to find service/ in SDK")
	}

	files, err := mocksDir.Readdir(0)
	if err != nil {
		return map[string]serviceName{}, errors.Wrap(err, "failed to read service/ in SDK")
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		dirName := f.Name()

		if _, dupe := avail[dirName]; dupe {
			continue
		}

		ifaceDir := filepath.Join(h.serviceDir, dirName, dirName+"iface")

		exists, _, err := cage_file.Exists(ifaceDir)
		if err != nil {
			return map[string]serviceName{}, errors.WithStack(err)
		}

		if !exists { // e.g. service package vendored but not the interface
			continue
		}

		svcName := serviceName{
			Package: dirName,
		}

		fname := filepath.Join(ifaceDir, "interface.go")
		src, err := ioutil.ReadFile(fname) // #nosec G304
		if err != nil {
			return map[string]serviceName{}, errors.Wrapf(err, "failed to read service [%s] interface file", dirName)
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, fname, src, parser.AllErrors)
		if err != nil {
			return map[string]serviceName{}, errors.Wrapf(err, "failed to parse service [%s] interface file", dirName)
		}

		ifaceName, err := h.getServiceIfaceName(f)
		if err != nil {
			return map[string]serviceName{}, errors.Wrapf(err, "failed to find service [%s] interface name", dirName)
		}
		svcName.InterfacePackage = dirName + "iface"
		svcName.InterfaceShort = ifaceName
		svcName.Interface = svcName.InterfacePackage + "." + ifaceName
		svcName.Proper = strings.TrimSuffix(ifaceName, "API")
		avail[dirName] = svcName
	}

	return avail, nil
}

// getServiceIfaceName returns the name (ast.Ident) of the first declared interface
// found in the node recursively.
//
// It's brittle in that it expects aws-sdk-go to continue its interface.go conventions.
func (h *Handler) getServiceIfaceName(node ast.Node) (string, error) {
	// Keep track of what we've found so that we know our progress.
	foundType := false
	name := ""
	done := false

	ast.Inspect(node, func(n ast.Node) bool {
		if done {
			return false
		}
		switch x := n.(type) {
		case *ast.TypeSpec:
			foundType = true
		case *ast.Ident:
			if foundType {
				name = x.String()
			}
		case *ast.InterfaceType:
			if name != "" {
				done = true
			}
		}
		return !done
	})
	if done {
		return name, nil
	}
	return "", errors.New("no interface declaration found")
}

var _ handler_cobra.Handler = (*Handler)(nil)
