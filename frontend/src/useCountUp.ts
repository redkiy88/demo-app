import { useEffect, useState } from "react";

// One-time reveal: counts from 0 to `target` over `durationMs`. Motion here
// answers the data arriving, not decoration on every render.
export function useCountUp(target: number | null, durationMs = 900): number {
  const [value, setValue] = useState(0);

  useEffect(() => {
    if (target === null) return;

    const start = performance.now();
    const from = 0;
    let frame: number;

    const tick = (now: number) => {
      const progress = Math.min((now - start) / durationMs, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      setValue(from + (target - from) * eased);
      if (progress < 1) frame = requestAnimationFrame(tick);
    };

    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [target, durationMs]);

  return value;
}
