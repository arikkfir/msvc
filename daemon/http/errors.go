package http

import (
	"fmt"
)

type ErrHttp interface {
	Code() int
	Cause() error
}

type errHttp struct {
	code  int
	cause error
}

func (e *errHttp) Code() int {
	return e.code
}

func (e *errHttp) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%d: %s", e.code, e.cause.Error())
	} else {
		return fmt.Sprintf("%d", e.code)
	}
}

func (e *errHttp) Cause() error {
	return e.cause
}

func NewHttpError(code int, cause error) error {
	return &errHttp{code, cause}
}
