package adapter

import (
	"context"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestAdapter(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(nil) })
	})
	t.Run("non_func", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter("test") })
	})
	t.Run("in_variadic", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s ...string) {}) })
	})
	t.Run("in_one_arg", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s string) {}) })
	})
	t.Run("in_three_arg", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 string, s2 string, s3 string) {}) })
	})
	t.Run("in_1st_arg_non_context", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 string, s2 *struct{}) {}) })
	})
	t.Run("in_2nd_arg_non_pointer", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 string) {}) })
	})
	t.Run("in_2nd_arg_non_struct_pointer", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *string) {}) })
	})
	t.Run("out_one", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (string) { return "" }) })
	})
	t.Run("out_three", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (string, string, string) { return "", "", "" }) })
	})
	t.Run("out_1st_non_pointer", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (string, string) { return "", "" }) })
	})
	t.Run("out_1st_non_struct_pointer", func(t *testing.T) {
		s := ""
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (*string, string) { return &s, "" }) })
	})
	t.Run("out_2nd_non_interface", func(t *testing.T) {
		s := struct{}{}
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (*struct{}, string) { return &s, "" }) })
	})
	t.Run("out_2nd_non_error", func(t *testing.T) {
		s := struct{}{}
		type I interface{}
		require.Panics(t, func() { _ = NewAdapter(func(s1 context.Context, s2 *struct{}) (*struct{}, I) { return &s, nil }) })
	})
	t.Run("works", func(t *testing.T) {
		type In struct{ P string }
		type Out struct{ P string }
		method := func(s1 context.Context, s2 *In) (*Out, error) { return &Out{P: "output"}, nil }
		adapter := NewAdapter(method)
		require.Equal(t, reflect.TypeOf(In{}), adapter.requestType)
		require.Equal(t, reflect.TypeOf(Out{}), adapter.responseType)
	})
}
