// OpenTelemetry initialization for the frontend
import { trace, context, propagation, SpanStatusCode } from '@opentelemetry/api'
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { resourceFromAttributes } from '@opentelemetry/resources'
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions'

/**
 * Initialize OpenTelemetry tracing for the frontend.
 * Sends spans to the Alloy collector via /otlp proxy.
 */
export function initTelemetry() {
  const resource = resourceFromAttributes({
    [ATTR_SERVICE_NAME]: 'kensan-frontend',
    [ATTR_SERVICE_VERSION]: 'dev',
  })

  const exporter = new OTLPTraceExporter({
    url: '/otlp/v1/traces',
  })

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(exporter)],
  })

  provider.register()
}

/**
 * Get the frontend tracer instance.
 */
export function getTracer() {
  return trace.getTracer('kensan-frontend')
}

/**
 * Inject W3C trace context headers (traceparent, tracestate) into a headers object.
 */
export function injectTraceHeaders(headers: Record<string, string>): Record<string, string> {
  propagation.inject(context.active(), headers, {
    set(carrier, key, value) {
      carrier[key] = value
    },
  })
  return headers
}

export { trace, SpanStatusCode }
