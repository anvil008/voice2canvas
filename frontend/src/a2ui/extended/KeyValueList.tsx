import { TONE_COLOR, type Tone } from "./types";

export interface KeyValueRow {
  label: string;
  value: string | number;
  tone?: Tone;
}

export function KeyValueList({ rows }: { rows: KeyValueRow[] }) {
  if (!Array.isArray(rows) || rows.length === 0) return <span className="extended-empty">No details</span>;
  return (
    <dl className="extended-key-values">
      {rows.map((row, index) => (
        <div key={`${row.label}-${index}`}>
          <dt>{row.label}</dt>
          <dd style={{ color: TONE_COLOR[row.tone ?? "neutral"] }}>{row.value}</dd>
        </div>
      ))}
    </dl>
  );
}
