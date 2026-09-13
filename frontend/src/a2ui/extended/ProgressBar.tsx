/**
 * Ported from Seaglass frontend/src/registry/ProgressBarRow.tsx; adapted to a
 * single catalog value on the fixed 0–100 domain.
 */
import { clamp, TONE_COLOR, type Tone } from "./types";

export function ProgressBar({
  label,
  value,
  tone = "neutral",
}: {
  label?: string;
  value: number;
  tone?: Tone;
}) {
  const percent = clamp(value, 0, 100);
  return (
    <div className="extended-progress" data-tone={tone}>
      <div className="extended-progress__heading">
        {label ? <span>{label}</span> : <span />}
        <strong style={{ color: TONE_COLOR[tone] }}>{Math.round(percent)}%</strong>
      </div>
      <div className="extended-progress__track" data-track="">
        <div
          className="extended-progress__fill"
          data-bar-fill=""
          style={{ background: TONE_COLOR[tone], width: `${percent}%` }}
        />
      </div>
    </div>
  );
}
