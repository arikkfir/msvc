package http

import (
	"github.com/arikkfir/msvc"
	"github.com/pkg/errors"
	"net/http"
)

type Handler interface {
	Handle(http.ResponseWriter, *http.Request)
}

type handler struct {
	requestDecoder  RequestDecoder
	responseEncoder ResponseEncoder
	methodAdapter   msvc.MethodAdapter
}

func NewHandler(methodAdapter msvc.MethodAdapter) *handler {
	requestDecoder, err := newRequestDecoder(methodAdapter.RequestType())
	if err != nil {
		panic(errors.Wrapf(err, "failed creating request decoder for '%s'", methodAdapter.RequestType()))
	}
	responseEncoder, err := newResponseEncoder(methodAdapter.ResponseType())
	if err != nil {
		panic(errors.Wrapf(err, "failed creating response encoder for '%s'", methodAdapter.ResponseType()))
	}
	return &handler{requestDecoder, responseEncoder, methodAdapter}
}

func (h *handler) Handle(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rvr := recover(); rvr != nil {
			if ms := msvc.GetFromContext(r.Context()); ms != nil {
				ms.Log("panic", rvr, "msg", "recovered from panic")
			}
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	serviceRequest, err := h.requestDecoder.Decode(r)
	if err != nil {
		h.responseEncoder.MarshallServiceResponseAndError(nil, err, r, w)
		return
	}

	serviceResponse, err := h.methodAdapter.Call(r.Context(), serviceRequest)
	if err != nil {
		h.responseEncoder.MarshallServiceResponseAndError(serviceResponse, err, r, w)
		return
	}

	h.responseEncoder.MarshallServiceResponse(serviceResponse, r, w)
}
