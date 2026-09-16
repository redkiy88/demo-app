import type { Mood } from "./weatherMood";

interface SkyProps {
  mood: Mood;
  isDay: boolean;
}

// The sky itself is the interface: a full-bleed animated scene driven by the
// real weather code + day/night flag, not an icon inside a card.
export default function Sky({ mood, isDay }: SkyProps) {
  const skyClass = `sky sky--${mood} ${isDay ? "sky--day" : "sky--night"}`;

  return (
    <div className={skyClass} aria-hidden="true">
      {!isDay && mood === "clear" && (
        <div className="stars">
          {Array.from({ length: 40 }).map((_, i) => (
            <span
              key={i}
              className="star"
              style={{
                top: `${(i * 37) % 90}%`,
                left: `${(i * 53) % 100}%`,
                animationDelay: `${(i % 10) * 0.4}s`,
              }}
            />
          ))}
        </div>
      )}

      {isDay && mood === "clear" && <div className="sun" />}
      {!isDay && mood === "clear" && <div className="moon" />}

      {(mood === "cloudy" || mood === "rain" || mood === "storm" || mood === "fog") && (
        <div className="clouds">
          <div className="cloud cloud--1" />
          <div className="cloud cloud--2" />
          <div className="cloud cloud--3" />
        </div>
      )}

      {mood === "fog" && <div className="fog-layer" />}

      {(mood === "rain" || mood === "storm") && (
        <div className="rain">
          {Array.from({ length: 60 }).map((_, i) => (
            <span
              key={i}
              className="raindrop"
              style={{
                left: `${(i * 13) % 100}%`,
                animationDelay: `${(i % 20) * 0.09}s`,
                animationDuration: `${0.5 + (i % 5) * 0.08}s`,
              }}
            />
          ))}
        </div>
      )}

      {mood === "snow" && (
        <div className="snow">
          {Array.from({ length: 40 }).map((_, i) => (
            <span
              key={i}
              className="snowflake"
              style={{
                left: `${(i * 17) % 100}%`,
                animationDelay: `${(i % 15) * 0.5}s`,
                animationDuration: `${6 + (i % 6)}s`,
              }}
            />
          ))}
        </div>
      )}
    </div>
  );
}
