import { trace } from "@opentelemetry/api";

export interface WhereamiResponse {
  ip: string;
  country: string;
  city: string;
  latitude: number;
  longitude: number;
  temperature_c: number;
  windspeed_kmh: number;
  weather_code: number;
  is_day: boolean;
}

export interface WhereamiResult {
  data: WhereamiResponse;
  traceId: string;
}

const tracer = trace.getTracer("geo-weather-frontend");

// Wrapped in an explicit span (rather than relying solely on the fetch
// auto-instrumentation) so we can hand the trace ID back to the UI - lets
// you find this exact request in Tempo afterwards.
export async function fetchWhereami(): Promise<WhereamiResult> {
  return tracer.startActiveSpan("whereami-request", async (span) => {
    const traceId = span.spanContext().traceId;
    try {
      const res = await fetch("/api/whereami");
      if (!res.ok) {
        throw new Error(`request failed: ${res.status}`);
      }
      const data = (await res.json()) as WhereamiResponse;
      return { data, traceId };
    } finally {
      span.end();
    }
  });
}
