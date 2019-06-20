package middleware

import (
	"context"
	"fmt"
	"github.com/arikkfir/msvc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type MetricsConfig struct {
	Port uint16
}

func NewMetricsServer(config *MetricsConfig) msvc.Daemon {
	return func() error {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		metricsHttpServer := &http.Server{Addr: fmt.Sprintf(":%d", config.Port), Handler: metricsMux}
		if err := metricsHttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		} else {
			return nil
		}
	}
}

func MethodDuration(ms *msvc.MicroService, methodName string, method msvc.Method) msvc.Method {
	metric := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "services",
		Subsystem: ms.Name(),
		Name:      "service_method_duration_seconds",
		Help:      "Duration of service method invocations.",
	}, []string{"service", "method"})

	prometheus.Unregister(metric)
	if err := prometheus.DefaultRegisterer.Register(metric); err != nil {
		panic(err)
	}

	return func(ctx context.Context, request interface{}) (interface{}, error) {
		defer func(begin time.Time) {
			metric.
				With(prometheus.Labels{"service": ms.Name(), "method": methodName}).
				Observe(time.Since(begin).Seconds())
		}(time.Now())
		return method(ctx, request)
	}
}
