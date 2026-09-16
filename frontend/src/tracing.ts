import { WebTracerProvider } from "@opentelemetry/sdk-trace-web";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-web";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { ZoneContextManager } from "@opentelemetry/context-zone";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";

// The browser can't reach cluster-internal Tempo directly, so spans are
// exported through gateway-service's /api/traces proxy (same origin,
// no CORS needed) which forwards them to Tempo's OTLP/HTTP receiver.
const exporter = new OTLPTraceExporter({
  url: "/api/traces",
});

const provider = new WebTracerProvider({
  resource: resourceFromAttributes({
    [ATTR_SERVICE_NAME]: "geo-weather-frontend",
  }),
  spanProcessors: [new BatchSpanProcessor(exporter)],
});

provider.register({
  contextManager: new ZoneContextManager(),
});

registerInstrumentations({
  instrumentations: [
    new FetchInstrumentation({
      // Propagate traceparent only to our own API, never to third parties.
      propagateTraceHeaderCorsUrls: [/\/api\//],
    }),
  ],
});
