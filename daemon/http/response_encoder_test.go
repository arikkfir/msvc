package http

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestResponseEncoder(t *testing.T) {
	t.Run("accept_header_missing", func(t *testing.T) {
		encoder, err := newResponseEncoder(reflect.TypeOf(""))
		require.NoError(t, err)

		response := httptest.NewRecorder()
		encoder.serviceResponseToHttpResponse(
			"Hello",
			httptest.NewRequest(http.MethodGet, url, nil),
			response)
		require.Equal(t, http.StatusNotAcceptable, response.Code)
		require.Equal(t, response.Body.String(), "")
	})
	t.Run("nil_response_is_ok", func(t *testing.T) {
		encoder, err := newResponseEncoder(reflect.TypeOf(""))
		require.NoError(t, err)

		response := httptest.NewRecorder()
		encoder.serviceResponseToHttpResponse(
			nil,
			httptest.NewRequest(http.MethodGet, url, nil),
			response)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, response.Body.String(), "")
	})
	t.Run("accept_json", func(t *testing.T) {
		type Resp struct {String string `json:"p"`}
		encoder, err := newResponseEncoder(reflect.TypeOf(&Resp{}))
		require.NoError(t, err)

		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, url, nil)
		request.Header.Add("accept", "application/json")
		encoder.serviceResponseToHttpResponse(Resp{String:"Hello"}, request, response)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, response.Body.String(), "{\n  \"p\": \"Hello\"\n}\n")
	})
}
