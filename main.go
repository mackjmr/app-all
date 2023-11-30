package main

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	mc "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	tracer  trace.Tracer
	counter mc.Int64Counter
	meter   mc.Meter
)

func initLogger() {
	f, err := os.OpenFile("/var/log/app/app.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05-0700",
	})
	log.SetOutput(f)
	log.SetLevel(log.InfoLevel)
}

func initMeter(ctx context.Context, r *resource.Resource) *metric.MeterProvider {
	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
	if err != nil {
		log.Fatalf("new otlp metric grpc exporter failed: %v", zap.Error(err))
	}
	provider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(exporter)), metric.WithResource(r))
	return provider
}

func initTracerProvider(ctx context.Context, r *resource.Resource) *sdktrace.TracerProvider {
	// Create exporter.
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to construct new exporter: %v", err)
	}

	// Create tracer provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
	)

	// Set tracer provider and propagator.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp
}
func main() {
	initLogger()

	ctx := context.Background()
	// Create resource.
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", "otel-docs")),
		resource.WithAttributes(attribute.String("deployment.environment", "dev")),
		resource.WithAttributes(attribute.String("service.version", "0.1")),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}
	tp := initTracerProvider(ctx, res)
	tracer = tp.Tracer("client-tracer")

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()
	mp := initMeter(ctx, res)
	defer func() {
		if err := mp.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down meter provider: %v", err)
		}
	}()

	meter = mp.Meter("otel-docs")
	counter, err = meter.Int64Counter("otel.docs.custom.metric")
	if err != nil {
		panic(err)
	}
	for {
		makeMetricSpanLog()
		time.Sleep(5 * time.Second)
	}

}

func makeMetricSpanLog() {
	_, span := tracer.Start(context.Background(), "work")
	log.WithFields(map[string]interface{}{
		"service":     "otel-docs",
		"env":         "dev",
		"version":     "0.1",
		"source":      "app",
		"trace_id": span.SpanContext().TraceID().String(),
	}).Info("Did Work")
	span.End()
	counter.Add(context.Background(), 1)
}
