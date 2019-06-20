package msvc

import (
	"context"
	"github.com/pkg/errors"
	"reflect"
)

type MethodAdapter interface {
	Call(ctx context.Context, request interface{}) (interface{}, error)
	RequestType() reflect.Type
	ResponseType() reflect.Type
}

type methodAdapter struct {
	target            interface{}
	targetMethodType  reflect.Type
	targetMethodValue reflect.Value
	requestType       reflect.Type
	responseType      reflect.Type
}

func NewAdapter(method interface{}) MethodAdapter {
	t := reflect.TypeOf(method)
	v := reflect.ValueOf(method)

	expectedSig := "func(context.Context, *<YourRequestStruct>)(*<YourResponseStruct>,error)"
	foundSig := t.String()
	if t.Kind() != reflect.Func {
		panic(errors.Errorf("not a function (%s)", method))
	} else if t.IsVariadic() || t.NumIn() != 2 {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if !t.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		panic(errors.Errorf("wrong signature - must be %s", expectedSig))
	} else if t.In(1).Kind() != reflect.Ptr {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if t.In(1).Elem().Kind() != reflect.Struct {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if t.Out(0).Kind() != reflect.Ptr {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if t.Out(0).Elem().Kind() != reflect.Struct {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if t.Out(1).Kind() != reflect.Interface {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	} else if !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic(errors.Errorf("wrong signature - must be %s, found: %s", expectedSig, foundSig))
	}

	return &methodAdapter{
		target:            method,
		targetMethodType:  t,
		targetMethodValue: v,
		requestType:       t.In(1).Elem(),
		responseType:      t.Out(0).Elem(),
	}
}

func (a *methodAdapter) Call(ctx context.Context, request interface{}) (interface{}, error) {
	// Create a pointer to the service request struct; for example:
	//  - "request" parameter in this context is MyServiceRequest{...} (it is NOT a pointer!)
	//  - service method signature is (must be): myService(context.Context, *MyServiceRequest)
	// We have "MyServiceRequest" and we create a "*MyServiceRequest" :)
	ptrValue := reflect.New(a.requestType)
	ptrValue.Elem().Set(reflect.ValueOf(request))

	// Create a slice with two items: context.Context and a pointer to the service request struct
	in := make([]reflect.Value, 0)
	in = append(in, reflect.ValueOf(ctx))
	in = append(in, ptrValue)

	// Call!
	returnValues := a.targetMethodValue.Call(in)

	var serviceResponse interface{}
	var errorResponse error

	if returnValues[0].IsNil() {
		serviceResponse = nil
	} else {
		serviceResponse = returnValues[0].Interface()
	}
	if returnValues[1].IsNil() {
		errorResponse = nil
	} else {
		errorResponse = returnValues[1].Interface().(error)
	}
	return serviceResponse, errorResponse
}

func (a *methodAdapter) RequestType() reflect.Type {
	return a.requestType
}

func (a *methodAdapter) ResponseType() reflect.Type {
	return a.responseType
}
