package http

import (
	"context"
	"github.com/bluebudgetz/msvc"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandler(t *testing.T) {
	t.Run("panics_on_bad_request_type", func(t *testing.T) {
		type Req struct{ P string } // missing 'http' tag
		type Res struct{}
		f := func(ctx context.Context, req *Req) (*Res, error) {
			panic("not implemented")
		}
		adapter := msvc.NewAdapter(f)
		require.Panics(t, func() { _ = NewHandler(adapter) })
	})
}

type errorReturningDecoder struct{}

func (d *errorReturningDecoder) Decode(r *http.Request) (interface{}, error) {
	return nil, errors.Errorf("bad")
}

func TestHandlerHandle(t *testing.T) {
	t.Run("decoder_failure", func(t *testing.T) {
		type Req struct{}
		type Res struct{}
		f := func(ctx context.Context, req *Req) (*Res, error) {
			panic("this will never happen (decoder will fail first)")
		}
		adapter := msvc.NewAdapter(f)
		handler := NewHandler(adapter)
		handler.requestDecoder = &errorReturningDecoder{}

		request := httptest.NewRequest("GET", "http://localhost:3001", nil)
		response := httptest.NewRecorder()
		handler.Handle(response, request)
		require.Equal(t, http.StatusInternalServerError, response.Code)
	})
	t.Run("method_error", func(t *testing.T) {
		type Req struct{}
		type Res struct{ P string }
		f := func(ctx context.Context, req *Req) (*Res, error) {
			return &Res{P: "v"}, errors.Errorf("bad")
		}
		adapter := msvc.NewAdapter(f)
		handler := NewHandler(adapter)
		request := httptest.NewRequest("GET", "http://localhost:3001", nil)
		request.Header.Set("accept", "application/json")
		response := httptest.NewRecorder()
		handler.Handle(response, request)
		require.Equal(t, http.StatusInternalServerError, response.Code)
		require.Equal(t, "application/json", response.Header().Get("content-type"))
		require.Equal(t, "{\n  \"P\": \"v\"\n}\n", response.Body.String())
	})
	t.Run("method_panic", func(t *testing.T) {
		type Req struct{}
		type Res struct{ P string }
		f := func(ctx context.Context, req *Req) (*Res, error) {
			panic("bad")
		}
		adapter := msvc.NewAdapter(f)
		handler := NewHandler(adapter)
		request := httptest.NewRequest("GET", "http://localhost:3001", nil)
		response := httptest.NewRecorder()
		handler.Handle(response, request)
		require.Equal(t, http.StatusInternalServerError, response.Code)
		require.Equal(t, "", response.Body.String())
	})
	t.Run("method_success", func(t *testing.T) {
		type Req struct{}
		type Res struct{ P string }
		f := func(ctx context.Context, req *Req) (*Res, error) {
			return &Res{P: "v"}, nil
		}
		adapter := msvc.NewAdapter(f)
		handler := NewHandler(adapter)
		request := httptest.NewRequest("GET", "http://localhost:3001", nil)
		request.Header.Set("accept", "application/json")
		response := httptest.NewRecorder()
		handler.Handle(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "application/json", response.Header().Get("content-type"))
		require.Equal(t, "{\n  \"P\": \"v\"\n}\n", response.Body.String())
	})
}
