package http

import (
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const url = "http://localhost:3001"

func TestRequestDecoder(t *testing.T) {
	t.Run("require_struct", func(t *testing.T) {
		_, err := newRequestDecoder(reflect.TypeOf(0))
		require.EqualError(t, err, "expected struct for request decoder target type; received 'int'")
	})
	t.Run("require_http_tags", func(t *testing.T) {
		_, err := newRequestDecoder(reflect.TypeOf(struct {
			FieldWithTag    string `http:"query,f"`
			FieldWithoutTag string
		}{}))
		require.EqualError(t, err, "missing 'http' tag for field 'FieldWithoutTag'")
	})
	t.Run("validate_http_tags", func(t *testing.T) {
		var err error
		_, err = newRequestDecoder(reflect.TypeOf(struct{ FieldWithInvalidTag string `http:""` }{}))
		require.EqualError(t, err, "illegal 'http' tag for field 'FieldWithInvalidTag': no tokens")
		_, err = newRequestDecoder(reflect.TypeOf(struct{ FieldWithInvalidTag string `http:"a,b,c"` }{}))
		require.EqualError(t, err, "illegal 'http' tag for field 'FieldWithInvalidTag': a,b,c")
	})
	t.Run("body", func(t *testing.T) {
		t.Run("json", func(t *testing.T) {
			t.Run("string", func(t *testing.T) {
				type ServiceRequest struct{ Body string `http:"body"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, strings.NewReader(`"hello"`))
				request.Header.Add("content-type", "application/json")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: "hello"}, result)
			})
			t.Run("*string", func(t *testing.T) {
				type ServiceRequest struct{ Body *string `http:"body"` }
				value := "hello"
				valueJson := fmt.Sprintf(`"%s"`, value)

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				var request *http.Request
				var result interface{}

				request = httptest.NewRequest(http.MethodGet, url, strings.NewReader(valueJson))
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: &value}, result)

				request = httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: nil}, result)

				request = httptest.NewRequest(http.MethodGet, url, strings.NewReader("null"))
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: nil}, result)
			})
			t.Run("struct", func(t *testing.T) {
				type Body struct {
					String       string  `json:"s"`
					StringPtr    *string `json:"sp"`
					NilStringPtr *string `json:"nsp"`
					Int          int     `json:"i"`
				}
				type ServiceRequest struct{ Body Body `http:"body"` }
				sp := "sp"
				json := fmt.Sprintf(`{"s":"s","i":9,"sp":"%s","nsp":null}`, sp)

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, strings.NewReader(json))
				request.Header.Add("content-type", "application/json")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: Body{String: "s", Int: 9, StringPtr: &sp, NilStringPtr: nil}}, result)
			})
			t.Run("*struct", func(t *testing.T) {
				type Body struct {
					String       string  `json:"s"`
					StringPtr    *string `json:"sp"`
					NilStringPtr *string `json:"nsp"`
					Int          int     `json:"i"`
				}
				type ServiceRequest struct{ Body *Body `http:"body"` }
				sp := "sp"
				json := fmt.Sprintf(`{"s":"s","i":9,"sp":"%s","nsp":null}`, sp)

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				var request *http.Request
				var result interface{}

				request = httptest.NewRequest(http.MethodGet, url, strings.NewReader(json))
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: &Body{String: "s", Int: 9, StringPtr: &sp, NilStringPtr: nil}}, result)

				request = httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: nil}, result)

				request = httptest.NewRequest(http.MethodGet, url, strings.NewReader("null"))
				request.Header.Add("content-type", "application/json")
				result, err = decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{Body: nil}, result)
			})
			t.Run("invalid", func(t *testing.T) {
				type ServiceRequest struct{ Body string `http:"body"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, strings.NewReader(`"unterminated`))
				request.Header.Add("content-type", "application/json")
				_, err = decoder.Decode(request)
				require.EqualError(t, err, "failed parsing request: failed reading JSON into 'string': unexpected EOF")
			})
		})
		t.Run("unsupported_media_type", func(t *testing.T) {
			type ServiceRequest struct{ Body string `http:"body"` }

			decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
			require.NoError(t, err)

			request := httptest.NewRequest(http.MethodGet, url, nil)
			request.Header.Add("content-type", "application/unknown")
			_, err = decoder.Decode(request)
			require.EqualError(t, err, "failed parsing request: 415: 'application/unknown' is not supported")
		})
	})
	t.Run("query", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"query,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: ""}, result)
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"query,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1", nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"query,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1&s=val2", nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
		})
		t.Run("*string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"query,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: nil}, result)
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"query,s"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s="+value, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"query,s"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1&s=val2", nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
		})
		t.Run("[]string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"query,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: nil}, result)
			})
			t.Run("single_value_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"query,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1", nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: []string{"val1"}}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"query,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1&s=val2", nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: []string{"val1", "val2"}}, result)
			})
		})
	})
	t.Run("path", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"path,p"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{Keys: []string{}, Values: []string{}},
				}))
				_, err = decoder.Decode(request)
				require.EqualError(t, err, "failed parsing request: empty value for path parameter 'p', required for field 'String'")
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"path,p"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001/v", nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{
						Keys:   []string{"p"},
						Values: []string{"val1"},
					},
				}))
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
		})
		t.Run("*string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"path,p"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{Keys: []string{}, Values: []string{}},
				}))
				_, err = decoder.Decode(request)
				require.EqualError(t, err, "failed parsing request: empty value for path parameter 'p', required for field 'String'")
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"path,p"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s="+value, nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{
						Keys:   []string{"p"},
						Values: []string{value},
					},
				}))
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
		})
		t.Run("[]string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"path,p"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{Keys: []string{}, Values: []string{}},
				}))
				_, err = decoder.Decode(request)
				require.EqualError(t, err, "failed parsing request: empty value for path parameter 'p', required for field 'String'")
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"path,p"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, "http://localhost:3001?s=val1&s=val2", nil)
				request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, &chi.Context{
					URLParams: chi.RouteParams{
						Keys:   []string{"p"},
						Values: []string{"val1"},
					},
				}))
				require.Panics(t, func() { _, _ = decoder.Decode(request) })
			})
		})
	})
	t.Run("header", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"header,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: ""}, result)
			})
			t.Run("single_value_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"header,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"header,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				request.Header.Add("s", "val2")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
		})
		t.Run("*string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"header,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: nil}, result)
			})
			t.Run("single_value_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"header,s"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"header,s"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				request.Header.Add("s", "val2")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
		})
		t.Run("[]string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"header,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: nil}, result)
			})
			t.Run("single_value_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"header,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: []string{"val1"}}, result)
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"header,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.Header.Add("s", "val1")
				request.Header.Add("s", "val2")
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: []string{"val1", "val2"}}, result)
			})
		})
	})
	t.Run("cookie", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"cookie,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: ""}, result)
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String string `http:"cookie,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.AddCookie(&http.Cookie{Name: "s", Value: "val1"})
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: "val1"}, result)
			})
		})
		t.Run("*string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"cookie,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: nil}, result)
			})
			t.Run("value_found", func(t *testing.T) {
				type ServiceRequest struct{ String *string `http:"cookie,s"` }
				value := "val1"
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				request.AddCookie(&http.Cookie{Name: "s", Value: "val1"})
				result, err := decoder.Decode(request)
				require.NoError(t, err)
				require.Equal(t, ServiceRequest{String: &value}, result)
			})
		})
		t.Run("[]string", func(t *testing.T) {
			t.Run("value_missing", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"cookie,s"` }

				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				require.Panics(t, func() { _, _ = decoder.Decode(request) })
			})
			t.Run("single_value_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"cookie,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				require.Panics(t, func() { _, _ = decoder.Decode(request) })
			})
			t.Run("multiple_values_found", func(t *testing.T) {
				type ServiceRequest struct{ String []string `http:"cookie,s"` }
				decoder, err := newRequestDecoder(reflect.TypeOf(ServiceRequest{}))
				require.NoError(t, err)

				request := httptest.NewRequest(http.MethodGet, url, nil)
				require.Panics(t, func() { _, _ = decoder.Decode(request) })
			})
		})
	})
}
