"""OpenTelemetry initialization for Kensan AI service."""

from __future__ import annotations

import logging
from dataclasses import dataclass

logger = logging.getLogger(__name__)


@dataclass
class TelemetryConfig:
    """OpenTelemetry configuration."""

    enabled: bool = False
    collector_url: str = "localhost:4318"
    service_name: str = "kensan-ai"


_shutdown_funcs: list = []


class _FilteringSpanProcessor:
    """SSE ストリーミングのノイズスパンをエクスポート前にドロップ。

    FastAPI の自動計装は SSE yield ごとに "http send" スパンを生成するが、
    μs 単位で情報価値がないためフィルタリングする。
    """

    def __init__(self, delegate):
        self._delegate = delegate

    def on_start(self, span, parent_context=None):
        self._delegate.on_start(span, parent_context)

    def on_end(self, span):
        name = span.name
        if "http send" in name and "/api/v1/agent/stream" in name:
            return
        self._delegate.on_end(span)

    def shutdown(self):
        self._delegate.shutdown()

    def force_flush(self, timeout_millis=30000):
        return self._delegate.force_flush(timeout_millis)


def initialize_telemetry(config: TelemetryConfig) -> None:
    """Initialize OpenTelemetry TracerProvider, MeterProvider, and LoggerProvider.

    If config.enabled is False, this is a no-op.
    """
    if not config.enabled:
        logger.info("OpenTelemetry disabled")
        return

    try:
        from opentelemetry import trace, metrics
        from opentelemetry._logs import set_logger_provider
        from opentelemetry.exporter.otlp.proto.http.metric_exporter import (
            OTLPMetricExporter,
        )
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import (
            OTLPSpanExporter,
        )
        from opentelemetry.exporter.otlp.proto.http._log_exporter import (
            OTLPLogExporter,
        )
        from opentelemetry.sdk.metrics import MeterProvider
        from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
        from opentelemetry.sdk.resources import Resource, SERVICE_NAME
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
        from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
        from opentelemetry.sdk._logs.export import BatchLogRecordProcessor

        resource = Resource.create({
            SERVICE_NAME: config.service_name,
            "service.version": "dev",
            "deployment.environment": "development",
        })
        endpoint = f"http://{config.collector_url}"

        # Traces
        span_exporter = OTLPSpanExporter(endpoint=f"{endpoint}/v1/traces")
        tp = TracerProvider(resource=resource)
        batch_processor = BatchSpanProcessor(span_exporter)
        tp.add_span_processor(_FilteringSpanProcessor(batch_processor))
        trace.set_tracer_provider(tp)
        _shutdown_funcs.append(tp.shutdown)

        # Metrics
        metric_exporter = OTLPMetricExporter(endpoint=f"{endpoint}/v1/metrics")
        reader = PeriodicExportingMetricReader(metric_exporter)
        mp = MeterProvider(resource=resource, metric_readers=[reader])
        metrics.set_meter_provider(mp)
        _shutdown_funcs.append(mp.shutdown)

        # Logs
        log_exporter = OTLPLogExporter(endpoint=f"{endpoint}/v1/logs")
        lp = LoggerProvider(resource=resource)
        lp.add_log_record_processor(BatchLogRecordProcessor(log_exporter))
        set_logger_provider(lp)
        _shutdown_funcs.append(lp.shutdown)

        # Attach OTel handler to kensan_ai logger
        # This bridges Python logging → OTel logs with automatic trace context
        otel_handler = LoggingHandler(level=logging.INFO, logger_provider=lp)
        kensan_logger = logging.getLogger("kensan_ai")
        kensan_logger.addHandler(otel_handler)
        kensan_logger.setLevel(logging.INFO)

        logger.info(
            "OpenTelemetry initialized (collector=%s)", config.collector_url
        )
    except ImportError:
        logger.warning(
            "OpenTelemetry packages not installed, skipping initialization"
        )
    except Exception:
        logger.exception("Failed to initialize OpenTelemetry, continuing without it")


def instrument_fastapi(app) -> None:  # noqa: ANN001
    """Instrument FastAPI app with OpenTelemetry.

    Excludes /health endpoint from tracing.
    """
    try:
        from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

        FastAPIInstrumentor.instrument_app(
            app,
            excluded_urls="health",
        )
        logger.info("FastAPI instrumented with OpenTelemetry")
    except ImportError:
        logger.debug("opentelemetry-instrumentation-fastapi not installed, skipping")
    except Exception:
        logger.exception("Failed to instrument FastAPI")


def instrument_asyncpg() -> None:
    """Instrument asyncpg with OpenTelemetry."""
    try:
        from opentelemetry.instrumentation.asyncpg import AsyncPGInstrumentor

        AsyncPGInstrumentor().instrument()
        logger.info("asyncpg instrumented with OpenTelemetry")
    except ImportError:
        logger.debug("opentelemetry-instrumentation-asyncpg not installed, skipping")
    except Exception:
        logger.exception("Failed to instrument asyncpg")


def instrument_httpx() -> None:
    """Instrument httpx with OpenTelemetry."""
    try:
        from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

        HTTPXClientInstrumentor().instrument()
        logger.info("httpx instrumented with OpenTelemetry")
    except ImportError:
        logger.debug("opentelemetry-instrumentation-httpx not installed, skipping")
    except Exception:
        logger.exception("Failed to instrument httpx")


def get_tracer(name: str):
    """Get an OpenTelemetry tracer by name.

    Returns the real tracer if OTel is initialized, otherwise a no-op tracer.
    """
    try:
        from opentelemetry import trace

        return trace.get_tracer(name)
    except ImportError:
        return _NoOpTracer()


class _NoOpSpan:
    """No-op span for when OTel is not available."""

    def set_attribute(self, key: str, value) -> None:  # noqa: ANN001
        pass

    def set_status(self, status, description: str | None = None) -> None:  # noqa: ANN001
        pass

    def record_exception(self, exception: BaseException) -> None:
        pass

    def end(self) -> None:
        pass

    def __enter__(self):
        return self

    def __exit__(self, *args):
        pass


class _NoOpTracer:
    """No-op tracer for when OTel is not available."""

    def start_span(self, name: str, **kwargs) -> _NoOpSpan:  # noqa: ANN003
        return _NoOpSpan()

    def start_as_current_span(self, name: str, **kwargs):  # noqa: ANN003
        return _NoOpSpan()


class _NoOpCounter:
    """No-op counter for when OTel is not available."""

    def add(self, amount: int | float, attributes: dict | None = None) -> None:  # noqa: ANN001
        pass


class _NoOpHistogram:
    """No-op histogram for when OTel is not available."""

    def record(self, amount: int | float, attributes: dict | None = None) -> None:  # noqa: ANN001
        pass


def get_meter(name: str):
    """Get an OpenTelemetry meter by name."""
    try:
        from opentelemetry import metrics

        return metrics.get_meter(name)
    except ImportError:
        return None


# GenAI metrics (lazy-initialized on first use)
_genai_token_usage: _NoOpCounter | None = None
_genai_operation_duration: _NoOpHistogram | None = None
_genai_operation_count: _NoOpCounter | None = None


def get_genai_metrics() -> tuple:
    """Get GenAI metric instruments, creating them on first call.

    Returns:
        (token_usage counter, operation_duration histogram, operation_count counter)
    """
    global _genai_token_usage, _genai_operation_duration, _genai_operation_count

    if _genai_token_usage is not None:
        return _genai_token_usage, _genai_operation_duration, _genai_operation_count

    meter = get_meter("kensan-ai.genai")
    if meter is None:
        _genai_token_usage = _NoOpCounter()
        _genai_operation_duration = _NoOpHistogram()
        _genai_operation_count = _NoOpCounter()
    else:
        _genai_token_usage = meter.create_counter(
            "gen_ai.client.token.usage",
            description="Number of tokens consumed by GenAI operations",
            unit="{token}",
        )
        _genai_operation_duration = meter.create_histogram(
            "gen_ai.client.operation.duration",
            description="Duration of GenAI agent interactions",
            unit="s",
        )
        _genai_operation_count = meter.create_counter(
            "gen_ai.client.operation.count",
            description="Number of GenAI agent operations",
            unit="{operation}",
        )

    return _genai_token_usage, _genai_operation_duration, _genai_operation_count


def shutdown_telemetry() -> None:
    """Gracefully shutdown OpenTelemetry providers."""
    for fn in reversed(_shutdown_funcs):
        try:
            fn()
        except Exception:
            logger.exception("Error shutting down OTel provider")
    _shutdown_funcs.clear()
