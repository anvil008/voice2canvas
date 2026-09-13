import { useEffect, useRef } from "react";
import type { TranscriptLine } from "../reducer";

interface TranscriptPanelProps {
  open: boolean;
  transcripts: TranscriptLine[];
  onClose: () => void;
}

export function TranscriptPanel({ open, transcripts, onClose }: TranscriptPanelProps) {
  const scrollRef = useRef<HTMLOListElement>(null);

  useEffect(() => {
    const list = scrollRef.current;
    if (!open || !list) return;
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
    if (typeof list.scrollTo === "function") {
      list.scrollTo({ top: list.scrollHeight, behavior: reduceMotion ? "auto" : "smooth" });
    } else {
      list.scrollTop = list.scrollHeight;
    }
  }, [open, transcripts]);

  return (
    <section
      id="transcript-content"
      className="transcript-panel"
      data-open={open}
      aria-hidden={!open}
      aria-label="Conversation transcript"
    >
      <div className="transcript-header">
        <div>
          <h2>Conversation</h2>
          <p>Live voice transcript</p>
        </div>
        <button
          type="button"
          className="transcript-close"
          aria-label="Collapse conversation"
          onClick={onClose}
        >
          Collapse
        </button>
      </div>
      {transcripts.length === 0 ? (
        <p className="transcript-empty">No speech yet. Press the mic when you’re ready.</p>
      ) : (
        <ol ref={scrollRef} className="transcript-list" aria-live="polite">
          {transcripts.map((line) => (
            <li
              key={line.id}
              className={`${line.direction === "input" ? "from-user" : "from-assistant"}${line.final ? " final" : " interim"}`}
            >
              <span className="transcript-speaker">
                {line.direction === "input" ? "You" : "Assistant"}
                {!line.final ? <span className="interim-label">Listening…</span> : null}
              </span>
              <p>{line.text}</p>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
