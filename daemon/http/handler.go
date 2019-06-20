package http

import (
	"github.com/bluebudgetz/msvc"
	"github.com/pkg/errors"
	"net/http"
)

type Handler struct {
	requestDecoder  *requestDecoder
	responseEncoder *responseEncoder
	methodAdapter   msvc.MethodAdapter
}

func NewHandler(methodAdapter msvc.MethodAdapter) *Handler {
	requestDecoder, err := newRequestDecoder(methodAdapter.RequestType())
	if err != nil {
		panic(errors.Wrapf(err, "failed creating request decoder for '%s'", methodAdapter.RequestType()))
	}
	responseEncoder, err := newResponseEncoder(methodAdapter.ResponseType())
	if err != nil {
		panic(errors.Wrapf(err, "failed creating response encoder for '%s'", methodAdapter.ResponseType()))
	}
	return &Handler{requestDecoder, responseEncoder, methodAdapter}
}

func (h *Handler) handle(w http.ResponseWriter, r *http.Request) {
	serviceRequest, err := h.requestDecoder.decode(r)
	if err != nil {
		h.responseEncoder.serviceResponseAndErrorToHttpResponse(nil, err, r, w)
		return
	}

	ctx := r.Context()
	serviceResponse, err := h.methodAdapter.Call(ctx, serviceRequest)
	if err != nil {
		h.responseEncoder.serviceResponseAndErrorToHttpResponse(serviceResponse, err, r, w)
		return
	}

	h.responseEncoder.serviceResponseToHttpResponse(serviceResponse, r, w)
}
