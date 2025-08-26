// Package router contains API routing logic
package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/telemetry"
)

// Middleware configuration options
type middlewareConfig struct {
	skipPaths map[string]bool
}

type MiddlewareOption func(*middlewareConfig)

// getRoutePath extracts the route pattern from the context
func getRoutePath(ctx huma.Context) string {
	// Try to get the operation from context
	if op := ctx.Operation().Path; op != "" {
		return ctx.Operation().Path
	}

	// Fallback to URL path (less ideal for metrics as it includes path parameters)
	return ctx.URL().Path
}

func MetricTelemetryMiddleware(metrics *telemetry.Metrics, options ...MiddlewareOption) func(huma.Context, func(huma.Context)) {
	config := &middlewareConfig{
		skipPaths: make(map[string]bool),
	}

	for _, opt := range options {
		opt(config)
	}

	return func(ctx huma.Context, next func(huma.Context)) {
		path := ctx.URL().Path

		// Skip instrumentation for specified paths
		// extract the last part of the path to match against skipPaths
		pathParts := strings.Split(path, "/")
		pathToMatch := "/" + pathParts[len(pathParts)-1]
		if config.skipPaths[pathToMatch] || config.skipPaths[path] {
			next(ctx)
			return
		}

		start := time.Now()
		method := ctx.Method()
		routePath := getRoutePath(ctx)

		next(ctx)

		duration := time.Since(start).Seconds()
		statusCode := ctx.Status()

		// Combine common and custom attributes
		attrs := []attribute.KeyValue{
			attribute.String("method", method),
			attribute.String("path", routePath),
			attribute.Int("status_code", statusCode),
		}

		// Record metrics
		metrics.Requests.Add(ctx.Context(), 1, metric.WithAttributes(attrs...))

		if statusCode >= 400 {
			metrics.ErrorCount.Add(ctx.Context(), 1, metric.WithAttributes(attrs...))
		}

		metrics.RequestDuration.Record(ctx.Context(), duration, metric.WithAttributes(attrs...))
	}
}

// WithSkipPaths allows skipping instrumentation for specific paths
func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		for _, path := range paths {
			c.skipPaths[path] = true
		}
	}
}

// NewHumaAPI creates a new Huma API with all routes registered
func NewHumaAPI(cfg *config.Config, registry service.RegistryService, mux *http.ServeMux, metrics *telemetry.Metrics) huma.API {
	// Create Huma API configuration
	humaConfig := huma.DefaultConfig("MCP Registry API", "1.0.0")
	humaConfig.Info.Description = "A community driven registry service for Model Context Protocol (MCP) servers."
	// Disable $schema property in responses: https://github.com/danielgtaylor/huma/issues/230
	humaConfig.CreateHooks = []func(huma.Config) huma.Config{}

	// Create a new API using humago adapter for standard library
	api := humago.New(mux, humaConfig)

	// Add metrics middleware with options
	api.UseMiddleware(MetricTelemetryMiddleware(metrics,
		WithSkipPaths("/health", "/metrics", "/ping", "/docs"),
	))

	// Register routes for all API versions
	RegisterV0Routes(api, cfg, registry, metrics)

	// Add /metrics for Prometheus metrics using promhttp
	mux.Handle("/metrics", metrics.PrometheusHandler())

	// Add redirect from / to /docs
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/docs", http.StatusMovedPermanently)
		}
	})

	return api
}
