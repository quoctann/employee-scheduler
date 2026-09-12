// Package tracing wires the OpenTelemetry SDK to export spans via OTLP/gRPC
// to the LGTM stack (Tempo) running in the same k3s cluster.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

// Params configures the tracer provider.
type Params struct {
	Disabled    bool
	Endpoint    string
	ServiceName string
}

// Shutdown flushes and stops the tracer provider. Safe to call even when
// tracing was disabled (no-op).
type Shutdown func(context.Context) error

// Init sets the global TracerProvider and propagator. When p.Disabled is
// true, it leaves OTel's no-op defaults in place and returns a no-op
// shutdown.
func Init(ctx context.Context, p Params) (Shutdown, error) {
	if p.Disabled {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(p.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(p.ServiceName),
	))
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}
