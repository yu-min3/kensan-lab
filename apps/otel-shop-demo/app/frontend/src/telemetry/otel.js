import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { Resource } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { ZoneContextManager } from '@opentelemetry/context-zone';

// Initialize OpenTelemetry for browser
export function initTelemetry() {
  // Get OTLP endpoint from environment or use default
  const otlpEndpoint = import.meta.env.VITE_OTEL_EXPORTER_OTLP_ENDPOINT ||
    'http://otel-collector-opentelemetry-collector.monitoring.svc:4318/v1/traces';

  // Create resource with service information
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: 'otel-shop-frontend',
    [ATTR_SERVICE_VERSION]: '1.0.0',
    'service.namespace': 'otel-shop-demo',
  });

  // Create OTLP trace exporter
  const exporter = new OTLPTraceExporter({
    url: otlpEndpoint,
  });

  // Create tracer provider
  const provider = new WebTracerProvider({
    resource: resource,
  });

  // Add batch span processor
  provider.addSpanProcessor(new BatchSpanProcessor(exporter));

  // Register the provider
  provider.register({
    contextManager: new ZoneContextManager(),
  });

  // Register instrumentations
  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [
          /.*/,  // Allow all origins for demo purposes
        ],
        clearTimingResources: true,
        applyCustomAttributesOnSpan: (span, request, result) => {
          // Add custom attributes to spans
          if (request.url) {
            span.setAttribute('http.url', request.url);
          }
          if (result instanceof Response) {
            span.setAttribute('http.status_code', result.status);
          }
        },
      }),
    ],
  });

  console.log('OpenTelemetry initialized for frontend');
}
