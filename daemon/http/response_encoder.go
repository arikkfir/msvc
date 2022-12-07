package http

import (
	"bytes"
	"encoding/json"
	"github.com/arikkfir/msvc"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"reflect"
	"strings"
)

type ResponseEncoder interface {
	MarshallServiceResponse(interface{}, *http.Request, http.ResponseWriter)
	MarshallServiceResponseAndError(interface{}, error, *http.Request, http.ResponseWriter)
}

type responseEncoder struct {
	sourceType  reflect.Type
	marshallers []func(*http.Request, http.ResponseWriter, reflect.Value) error
}

func newResponseEncoder(sourceType reflect.Type) (*responseEncoder, error) {
	if sourceType.Kind() != reflect.Struct {
		return nil, errors.Errorf("expected struct for response encoder source type; received '%s'", sourceType.Kind())
	}

	marshallers := make([]func(*http.Request, http.ResponseWriter, reflect.Value) error, 0, 10)
	for i := 0; i < sourceType.NumField(); i++ {
		fieldType := sourceType.Field(i)

		tag, ok := fieldType.Tag.Lookup("http")
		if !ok {
			return nil, errors.Errorf("missing 'http' tag for field '%s'", fieldType.Name)
		}

		tokens := strings.Split(tag, ",")
		if len(tokens) == 0 || len(tokens) == 1 && strings.TrimSpace(tokens[0]) == "" {
			return nil, errors.Errorf("illegal 'http' tag for field '%s': no tokens", fieldType.Name)
		} else if len(tokens) == 1 && tokens[0] == "body" {
			marshallers = append(marshallers, newBodyDecoder(fieldType))
		} else if len(tokens) > 2 {
			return nil, errors.Errorf("illegal 'http' tag for field '%s': %s", fieldType.Name, tag)
		} else {
			if len(tokens) == 1 {
				tokens = []string{tokens[0], strings.ToLower(fieldType.Name)}
			}
			switch tokens[0] {
			case "query":
				marshallers = append(marshallers, newQueryParameterDecoder(fieldType, tokens[1]))
			case "path":
				marshallers = append(marshallers, newPathParameterDecoder(fieldType, tokens[1]))
			case "header":
				marshallers = append(marshallers, newHeaderDecoder(fieldType, tokens[1]))
			case "cookie":
				marshallers = append(marshallers, newCookieDecoder(fieldType, tokens[1]))
			default:
				return nil, errors.Errorf("illegal 'http' tag for field '%s': %s", fieldType.Name, tag)
			}
		}
	}
	return &responseEncoder{sourceType, marshallers}, nil
}

func (h *responseEncoder) marshallServiceResponse(ms *msvc.MicroService, serviceResponse interface{}, mediaType string, w io.Writer) error {
	if serviceResponse != nil {
		switch mediaType {
		case "application/json":
			encoder := json.NewEncoder(w)
			encoder.SetEscapeHTML(false)
			if ms == nil || ms.Environment() != msvc.EnvProduction {
				encoder.SetIndent("", "  ")
			}
			return encoder.Encode(serviceResponse)
		default:
			return NewHttpError(http.StatusNotAcceptable, errors.Errorf("'%s' is not supported", mediaType))
		}
	}
	return nil
}

func (h *responseEncoder) MarshallServiceResponse(serviceResponse interface{}, r *http.Request, w http.ResponseWriter) {
	if serviceResponse == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	ms := msvc.GetFromContext(r.Context())
	mediaType := r.Header.Get("accept")

	// Marshall service response into an in-memory buffer
	buffer := new(bytes.Buffer)
	if err := h.marshallServiceResponse(ms, serviceResponse, mediaType, buffer); err != nil {
		if ms != nil {
			ms.Log("err", err, "msg", "failed encoding JSON")
		}
		if httpErr, ok := err.(ErrHttp); ok {
			err = httpErr.Cause()
			if ms != nil {
				ms.Log("res", serviceResponse, "err", err)
			}
			w.WriteHeader(httpErr.Code())
		} else {
			if ms != nil {
				ms.Log("res", serviceResponse, "err", err)
			}
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Write buffer back to client
	w.Header().Set("Content-Type", mediaType)
	w.WriteHeader(http.StatusOK)
	if _, err := buffer.WriteTo(w); err != nil && ms != nil {
		ms.Log("err", err, "msg", "failed serializing JSON buffer to response")
	}
}

func (h *responseEncoder) MarshallServiceResponseAndError(serviceResponse interface{}, serviceError error, r *http.Request, w http.ResponseWriter) {
	ms := msvc.GetFromContext(r.Context())
	var httpStatusCode int

	// If given error specifies the HTTP code, send that back to client; otherwise, use HTTP 500 (Internal Error)
	// Also log the error
	if httpErr, ok := serviceError.(ErrHttp); ok {
		serviceError = httpErr.Cause()
		if ms != nil {
			ms.Log("res", serviceResponse, "err", serviceError)
		}
		httpStatusCode = httpErr.Code()
	} else {
		if ms != nil {
			ms.Log("res", serviceResponse, "err", serviceError)
		}
		httpStatusCode = http.StatusInternalServerError
	}

	// Write HTTP status code; if we have a service response, add "content-type" header too (must be done before writing HTTP status code)
	if serviceResponse != nil {
		w.Header().Set("content-type", r.Header.Get("accept"))
	}
	w.WriteHeader(httpStatusCode)

	// Marshall service response into an in-memory buffer, if any
	if serviceResponse != nil {
		if err := h.marshallServiceResponse(ms, serviceResponse, r.Header.Get("accept"), w); err != nil && ms != nil {
			ms.Log("res", serviceResponse, "err", err)
		}
	}
}
