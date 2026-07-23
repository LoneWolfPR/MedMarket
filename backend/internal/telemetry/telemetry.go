// Package telemetry holds the composition-root helper for running
// open telemetry within the application
package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Setup does all the work setting up open telemetry
func Setup(
	ctx context.Context,
	endpoint, serviceName string,
) (func(context.Context) error, error) {
	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("error building trace exporter: %w", err)
	}

	otelRes, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("error building resource: %w", err)
	}

	tp := trace.NewTracerProvider(trace.WithBatcher(traceExporter), trace.WithResource(otelRes))

	otel.SetTracerProvider(tp)

	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(propagator)

	logExporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("error building log exporter: %w", err)
	}

	lp := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
		log.WithResource(otelRes),
	)

	global.SetLoggerProvider(lp)

	return func(ctx context.Context) error {
		err := errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx))
		if err != nil {
			return fmt.Errorf("error shutting down: %w", err)
		}
		return nil
	}, nil

}
