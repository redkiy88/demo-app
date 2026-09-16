import { useEffect, useState } from "react";
import Sky from "./Sky";
import { fetchWhereami, type WhereamiResponse } from "./api";
import { moodFromCode, labelFromCode } from "./weatherMood";
import { useCountUp } from "./useCountUp";
import "./App.css";

type LoadState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; data: WhereamiResponse; traceId: string };

export default function App() {
  const [state, setState] = useState<LoadState>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;

    fetchWhereami()
      .then(({ data, traceId }) => {
        if (!cancelled) setState({ status: "ready", data, traceId });
      })
      .catch((err: Error) => {
        if (!cancelled) setState({ status: "error", message: err.message });
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const ready = state.status === "ready";
  const temperature = useCountUp(ready ? state.data.temperature_c : null);

  const mood = ready ? moodFromCode(state.data.weather_code) : "cloudy";
  const isDay = ready ? state.data.is_day : true;

  return (
    <div className="scene">
      <Sky mood={mood} isDay={isDay} />

      <main className="content">
        {state.status === "loading" && (
          <p className="status-line">Finding you on the map…</p>
        )}

        {state.status === "error" && (
          <div className="error-block">
            <p className="status-line">Couldn't reach the sky.</p>
            <p className="status-detail">{state.message}</p>
          </div>
        )}

        {state.status === "ready" && (
          <>
            <div className="place">
              <h1 className="place__city">{state.data.city || "Unknown city"}</h1>
              <p className="place__country">{state.data.country || "Unknown location"}</p>
            </div>

            <div className="reading">
              <p className="reading__temp">
                {Math.round(temperature)}
                <span className="reading__unit">°</span>
              </p>
              <p className="reading__label">{labelFromCode(state.data.weather_code)}</p>
              <p className="reading__wind">Wind {Math.round(state.data.windspeed_kmh)} km/h</p>
            </div>
          </>
        )}
      </main>

      {state.status === "ready" && (
        <footer className="trace-footer">trace {state.traceId}</footer>
      )}
    </div>
  );
}
