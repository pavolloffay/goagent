package hypertrace

import (
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"github.com/hypertrace/goagent/config"
	sdkconfig "github.com/hypertrace/goagent/sdk/config"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/exporters/trace/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

const batchTimeoutInSecs = 200.0

// Init initializes opentelemetry tracing and returns a shutdown function to flush data immediately
// on a termination signal.
func Init(cfg *config.AgentConfig) func() {
	sdkconfig.InitConfig(cfg)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !cfg.GetReporting().GetSecure().GetValue()},
	}}

	zipkinBatchExporter, err := zipkin.NewRawExporter(
		cfg.GetReporting().GetEndpoint().GetValue(),
		cfg.GetServiceName().GetValue(),
		zipkin.WithClient(client),
	)
	if err != nil {
		log.Fatal(err)
	}

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithBatcher(zipkinBatchExporter, sdktrace.WithBatchTimeout(batchTimeoutInSecs*time.Millisecond)),
		sdktrace.WithResource(
			resource.New(semconv.ServiceNameKey.String(cfg.GetServiceName().GetValue())),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	global.SetTraceProvider(tp)

	return func() {
		// This is a sad temporary solution for the lack of flusher in the batcher interface.
		// What we do here is that we wait for `batchTimeout` seconds as that is the time configured
		// in the batcher and hence we make sure spans had time to be flushed.
		// In next versions the flush functionality is finally added and we will use it.
		<-time.After(batchTimeoutInSecs * 1.5 * time.Millisecond)
	}
}