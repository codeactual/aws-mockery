// Copyright (C) 2019 The CodeActual Go Environment Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package require

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	std_require "github.com/stretchr/testify/require"

	cage_strings "github.com/codeactual/aws-mockery/internal/cage/strings"
)

// ReaderLineReplacer receives an "actual" line, from an expected vs. actual test comparison, and optionally
// returns a new "actual" line value.
//
// For example, if the actual values will change over time (e.g. "go <semver>" strings in new go.mod files),
// a ReaderLineReplacer can allow test fixtures to have out-dated semver values (which are not the SUT) by
// replacing the dynamic value with the expected static value.
//
// The expectedName and actualName values are the reader (e.g. file) names.
type ReaderLineReplacer func(expectedName, actualName, actualLine string) (replacement string)

func ReadersMatch(t *testing.T, expectedName string, expected io.Reader, actualName string, actual io.Reader, lrs ...ReaderLineReplacer) {
	expectedBytes, err := ioutil.ReadAll(expected)
	std_require.NoError(t, errors.WithStack(err))
	expectedLines := strings.Split(string(expectedBytes), "\n")

	actualBytes, err := ioutil.ReadAll(actual)
	std_require.NoError(t, errors.WithStack(err))
	actualLines := strings.Split(string(actualBytes), "\n")

	// Allow tests to work around lines which may change over time or differ between platforms.
	for _, lr := range lrs {
		for n := range actualLines {
			actualLines[n] = lr(expectedName, actualName, actualLines[n])
		}
	}

	// Add newlines for long paths
	std_require.Exactly(t, expectedLines, actualLines, "\nexpected [%s]\nactual [%s]", expectedName, actualName)
}

func StringSortedSliceExactly(t *testing.T, expected []string, actual []string) {
	e := make([]string, len(expected))
	copy(e, expected[:])
	cage_strings.SortStable(e)

	a := make([]string, len(actual))
	copy(a, actual[:])
	cage_strings.SortStable(a)

	StringSliceExactly(t, e, a)
}

func StringSliceExactly(t *testing.T, expected []string, actual []string) {
	std_require.Exactly(t, expected, actual, fmt.Sprintf(
		"expect: %s\nactual: %s\n", spew.Sdump(expected), spew.Sdump(actual),
	))
}
