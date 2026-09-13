import { type Tone } from "./types";

export function Badge({ text, tone = "neutral" }: { text: string; tone?: Tone }) {
  return <span className="extended-badge" data-tone={tone}>{text}</span>;
}
