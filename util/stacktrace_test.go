package util

import (
	"errors"
	"fmt"
	"github.com/kr/text"
	errors2 "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

var (
	testStackTraceRE = regexp.MustCompile("stacktrace_test.go:\\d+")
)

func TestFormatStackTraceWithString(t *testing.T) {
	errorString := "error string"
	expectedStackTrace := testStackTraceRE.ReplaceAllString(stackTrace(), "stacktrace_test.go:\\d+")
	require.Regexp(
		t,
		fmt.Sprintf("^%s\n%s$", errorString, text.Indent(expectedStackTrace, "    ")),
		formatStackTrace(errorString),
	)
}

func TestFormatStackTraceWithSimpleError(t *testing.T) {
	errorString := "error string"
	err := errors.New(errorString)
	expectedStackTrace := testStackTraceRE.ReplaceAllString(stackTrace(), "stacktrace_test.go:\\d+")
	require.Regexp(
		t,
		fmt.Sprintf("^%s\n%s$", errorString, text.Indent(expectedStackTrace, "    ")),
		formatStackTrace(err),
	)
}

func TestFormatStackTraceWithStackTraceProvidingError(t *testing.T) {
	errorString := "error string"
	err := errors2.New(errorString)
	expectedStackTrace := testStackTraceRE.ReplaceAllString(stackTrace(), "stacktrace_test.go:\\d+")
	require.Regexp(
		t,
		fmt.Sprintf("^%s\n%s$", errorString, text.Indent(expectedStackTrace, "    ")),
		formatStackTrace(err),
	)
}

func TestFormatStackTraceWithCausingError(t *testing.T) {
	rootString := "root error"
	causeString := "cause error"
	causeErr := errors2.New(causeString)
	rootErr := errors2.Wrap(causeErr, rootString)
	expectedStackTrace := testStackTraceRE.ReplaceAllString(stackTrace(), "stacktrace_test.go:\\d+")
	require.Regexp(
		t,
		fmt.Sprintf("^%s: %s\n%s\nCaused by: %s\n%s$",
			rootString,
			causeString,
			text.Indent(expectedStackTrace, "    "),
			causeString,
			text.Indent(expectedStackTrace, "    ")),
		formatStackTrace(rootErr),
	)
}
