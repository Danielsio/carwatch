package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds the settings for initialising OpenTelemetry tracing.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Exporter       string // "none", "stdout", "otlp"
	OTLPEndpoint   string
}

// Init bootstraps an OpenTelemetry TracerProvider based on cfg.Exporter.
// The returned shutdown function flushes pending spans and releases resources.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if cfg.Exporter == "none" || cfg.Exporter == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlp":
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("unknown traces exporter: %q", cfg.Exporter)
	}
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	// Sample 10 % of traces in production; parent-based preserves
	// upstream sampling decisions when present.
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("telemetry initialized", "exporter", cfg.Exporter)
	return tp.Shutdown, nil
}
