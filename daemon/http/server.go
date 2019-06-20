package http

import (
	"fmt"
	"github.com/arikkfir/msvc"
	"net/http"
)

type Config struct {
	Port uint16
	CORS struct {
		Host string
		Port uint16
	}
}

func NewHTTPServer(ms *msvc.MicroService, config *Config, handlers map[string]interface{}) msvc.Daemon {
	return func() error {
		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", config.Port),
			Handler: createRouter(ms, config.CORS.Host, config.CORS.Port, handlers),
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		} else {
			return nil
		}
	}
}
