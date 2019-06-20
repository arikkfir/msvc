package middleware

import (
	"context"
	"github.com/bluebudgetz/msvc"
)

func Logging(ms *msvc.MicroService, methodName string, method msvc.Method) msvc.Method {
	return func(ctx context.Context, request interface{}) (returnValue interface{}, err error) {
		defer func() {
			ms.Log("service", ms.Name(), "method", methodName, "request", request, "response", returnValue, "err", err)
		}()
		return method(ctx, request)
	}
}
