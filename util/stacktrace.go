package util

import (
	"fmt"
	kitlog "github.com/go-kit/kit/log"
	"github.com/kr/text"
	"github.com/pkg/errors"
	"io"
	"strings"
)

// Implemented by errors that provide access to their contextual stack-trace.
type stackTracerProvider interface {
	StackTrace() errors.StackTrace
}

// Implemented by errors that provide a wrapped error.
type causeProvider interface {
	Cause() error
}

// Returns a function that receives an array of key/value pairs, prints them, but also prints a stack-trace of an error,
// if one was provided in the key/value pairs.
func CreateStackTraceLoggerFunc(writer io.Writer, logger kitlog.Logger) kitlog.LoggerFunc {
	return func(kv ...interface{}) error {
		if err := logger.Log(kv...); err != nil {
			return err
		}
		for index, val := range kv {
			if index%2 == 1 {
				key := kv[index-1]
				if key == "panic" || key == "err" {
					stackTrace := formatStackTrace(val)
					if _, err := writer.Write([]byte(text.Indent(stackTrace, "  "))); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}

func stackTrace() string {
	inlineStackTrace := fmt.Sprintf("%+v", errors.New("dummyError"))
	lines := strings.Split(inlineStackTrace, "\n")
	return strings.Join(lines[3:], "\n")
}

func formatStackTrace(errorVal interface{}) string {
	var stackTrace = ""
	var root = true
	for errorVal != nil {
		if stackTrace != "" {
			stackTrace += "\nCaused by: "
		}
		if err, ok := errorVal.(error); ok {
			stackTrace += err.Error() + "\n"
		} else {
			stackTrace += fmt.Sprintf("%s\n", errorVal)
		}

		// If error provides a stacktrace, add it
		if stp, ok := errorVal.(stackTracerProvider); ok {
			stackTrace += text.Indent(strings.TrimSpace(fmt.Sprintf("%+v", stp.StackTrace())), "    ")
		} else if root {
			// only generate an ad-hoc stacktrace for the root error; causing errors in the chain WILL NOT get stacktraces
			inlineStackTrace := fmt.Sprintf("%+v", errors.New("dummyError"))
			lines := strings.Split(inlineStackTrace, "\n")
			stackTrace += text.Indent(strings.Join(lines[3:], "\n"), "    ")
		}
		root = false

		// If error has a cause, move on to that
		if err, ok := errorVal.(error); ok {
			oldMsg := err.Error()
			for err != nil && err.Error() == oldMsg {
				if causeProvider, ok := err.(causeProvider); ok {
					err = causeProvider.Cause()
				} else {
					err = nil
				}
			}
			errorVal = err
		} else {
			errorVal = nil
		}
	}
	return stackTrace
}
