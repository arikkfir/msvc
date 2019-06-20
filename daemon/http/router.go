package http

import (
	"fmt"
	"github.com/arikkfir/msvc"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"net/http"
	"strings"
)

func createRouter(ms *msvc.MicroService, corsHost string, corsPort uint16, handlers map[string]interface{}) chi.Router {

	// Create router
	router := chi.NewRouter()
	router.Use(
		// Provide the "server" header
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("server", "msvc")
				next.ServeHTTP(w, r)
			})
		},

		// First provide a "/health" endpoint
		middleware.Heartbeat("/health"),

		// Ensure request is uniquely identified & logged (with the real user IP)
		middleware.RequestID,
		middleware.RealIP,

		// Provide micro-service
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(msvc.SetInContext(r.Context(), ms)))
			})
		},

		// Recover panics, and replace them with HTTP 500 response
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() {
					if rvr := recover(); rvr != nil {
						ms.Log(
							"remoteAddr", r.RemoteAddr,
							"proto", r.Proto,
							"method", r.Method,
							"requestURI", r.RequestURI,
							"headers", fmt.Sprintf("%v", r.Header),
							"url", r.URL,
							"host", r.Host,
							"panic", rvr,
						)
						w.WriteHeader(http.StatusInternalServerError)
					}
				}()
				next.ServeHTTP(w, r)
			})
		},

		// Use "GET" handlers if "HEAD"-specific handlers are not found
		middleware.GetHead,

		// Apply common headers
		middleware.NoCache,
		middleware.AllowContentType("application/json", ""),
		middleware.ContentCharset("", "UTF-8"),

		// CORS
		cors.New(cors.Options{
			AllowedOrigins:   []string{fmt.Sprintf("http://%s:%d", corsHost, corsPort)},
			AllowedMethods:   []string{"OPTIONS", "HEAD", "GET", "POST", "PATCH", "PUT", "DELETE"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}).Handler,
	)

	// Register handlers
	for k, v := range handlers {
		mountRoutes(router, k, v)
	}

	return router
}

func mountRoutes(router chi.Router, entryKey string, entryValue interface{}) {
	upperCaseEntryKey := strings.ToUpper(entryKey)
	if routesMap, ok := entryValue.(map[string]interface{}); ok {
		if entryKey[0] != '/' {
			entryKey = "/" + entryKey
		}
		router.Route(entryKey, func(r chi.Router) {
			for k, v := range routesMap {
				mountRoutes(r, k, v)
			}
		})
	} else if upperCaseEntryKey == http.MethodGet ||
		upperCaseEntryKey == http.MethodHead ||
		upperCaseEntryKey == http.MethodPost ||
		upperCaseEntryKey == http.MethodPut ||
		upperCaseEntryKey == http.MethodPatch ||
		upperCaseEntryKey == http.MethodDelete ||
		upperCaseEntryKey == http.MethodConnect ||
		upperCaseEntryKey == http.MethodOptions ||
		upperCaseEntryKey == http.MethodTrace {

		if handler, ok := entryValue.(Handler); ok {
			router.MethodFunc(strings.ToUpper(entryKey), "/", handler.Handle)
		} else {
			panic(errors.Errorf("bad routes map in '%s: %+v'", entryKey, entryValue))
		}
	} else if handler, ok := entryValue.(Handler); ok {
		if entryKey[0] != '/' {
			entryKey = "/" + entryKey
		}
		router.HandleFunc(entryKey, handler.Handle)
	} else {
		panic(errors.Errorf("bad routes map in '%s: %+v'", entryKey, entryValue))
	}
}
