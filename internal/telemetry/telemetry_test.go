package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_None(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{
		Exporter: "none",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestInit_EmptyExporter(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{
		Exporter: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestInit_Stdout(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{
		ServiceName:    "test-svc",
		ServiceVersion: "0.0.1",
		Exporter:       "stdout",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	// Verify a real TracerProvider was installed.
	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Fatalf("expected *sdktrace.TracerProvider, got %T", tp)
	}
}

func TestInit_UnknownExporter(t *testing.T) {
	_, err := Init(context.Background(), Config{
		Exporter: "kafka",
	})
	if err == nil {
		t.Fatal("expected error for unknown exporter")
	}
}

func TestInit_OTLPBadEndpoint(t *testing.T) {
	// OTLP creation itself should succeed (connection is lazy),
	// but we exercise the code path.
	shutdown, err := Init(context.Background(), Config{
		ServiceName:  "test-svc",
		Exporter:     "otlp",
		OTLPEndpoint: "localhost:0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Shutdown is expected to succeed even without a real collector.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}
