package http

import (
	"bytes"
	"encoding/json"
	"github.com/bluebudgetz/msvc"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"reflect"
)

type responseEncoder struct {
	sourceType reflect.Type
}

func newResponseEncoder(sourceType reflect.Type) (*responseEncoder, error) {
	return &responseEncoder{sourceType}, nil
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

func (h *responseEncoder) serviceResponseToHttpResponse(serviceResponse interface{}, r *http.Request, w http.ResponseWriter) {
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
			w.WriteHeader(int(httpErr.Code()))
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

func (h *responseEncoder) serviceResponseAndErrorToHttpResponse(serviceResponse interface{}, serviceError error, r *http.Request, w http.ResponseWriter) {
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
