import { useEffect, useRef, useState } from "react";
import type { DashboardStatus, TranscriptLine } from "../reducer";
import type { ConnectionState } from "../ws";
import { ChevronIcon, MicIcon, MutedIcon, SparkIcon, StopIcon } from "./icons";
import { TranscriptPanel } from "./TranscriptPanel";

type HealthState = "checking" | "healthy" | "unavailable" | "mock";
type VoiceState = "idle" | "connecting" | "listening" | "muted" | "error" | "demo";

interface VoiceDockProps {
  connection: ConnectionState;
  health: HealthState;
  isMockMode: boolean;
  muted: boolean;
  started: boolean;
  starting: boolean;
  status: DashboardStatus;
  transcripts: TranscriptLine[];
  onMainAction: () => void;
  onMuteToggle: () => void;
  onStop: () => void;
}

function voiceState(props: VoiceDockProps): VoiceState {
  if (props.status.tone === "error") return "error";
  if (props.isMockMode) return "demo";
  if (props.connection === "closed" || props.health === "unavailable") return "error";
  if (
    props.starting ||
    props.connection === "reconnecting" ||
    (props.started && props.connection !== "connected")
  ) {
    return "connecting";
  }
  if (!props.started) return "idle";
  return props.muted ? "muted" : "listening";
}

const STATE_LABELS: Record<VoiceState, string> = {
  idle: "Ready to listen",
  connecting: "Connecting…",
  listening: "Listening",
  muted: "Microphone muted",
  error: "Needs attention",
  demo: "Mock playback",
};

export function VoiceDock(props: VoiceDockProps) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const transcriptToggleRef = useRef<HTMLButtonElement>(null);
  const state = voiceState(props);
  const socketLabel = props.isMockMode ? "mock" : props.connection;
  const healthLabel = props.health === "healthy" ? "online" : props.health;
  const latestFinal = [...props.transcripts].reverse().find((line) => line.final);
  const showTranscriptPeek = props.started && latestFinal !== undefined;
  const mainLabel = !props.started
    ? "Start voice session"
    : props.muted
      ? "Unmute microphone"
      : "Mute microphone";

  const closeDrawer = (restoreFocus = false): void => {
    setDrawerOpen(false);
    if (restoreFocus) {
      transcriptToggleRef.current?.focus();
    }
  };

  useEffect(() => {
    if (!drawerOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      closeDrawer(true);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [drawerOpen]);

  return (
    <section className="voice-dock-shell" data-drawer-open={drawerOpen} data-voice-state={state}>
      <TranscriptPanel
        open={drawerOpen}
        transcripts={props.transcripts}
        onClose={() => closeDrawer(true)}
      />

      <div className="voice-dock">
        <div className="voice-primary-wrap" data-testid="dock-mic-cluster">
          <span className="voice-pulse" aria-hidden="true" />
          <button
            type="button"
            className="voice-primary"
            aria-label={mainLabel}
            title={props.isMockMode ? "Voice controls are disabled in mock mode" : mainLabel}
            disabled={props.isMockMode || props.starting}
            onClick={props.onMainAction}
          >
            {state === "connecting" || state === "listening"
              ? <SparkIcon className="voice-spark" />
              : state === "muted"
                ? <MutedIcon />
                : <MicIcon />}
          </button>
        </div>

        <button
          className="voice-readout"
          type="button"
          aria-controls="transcript-content"
          aria-expanded={drawerOpen}
          aria-label="Voice status"
          data-testid="dock-status-cluster"
          onClick={() => setDrawerOpen((open) => !open)}
        >
          <span className="voice-state-line" role="status">
            <strong>{STATE_LABELS[state]}</strong>
          </span>
          <span className="voice-detail-line">
            <span
              className={`health-dot health-${props.health}`}
              title={`Backend ${healthLabel}; socket ${socketLabel}`}
            />
            <span
              className={`voice-status-detail${showTranscriptPeek ? " transcript-status-peek" : ""}`}
              title={showTranscriptPeek ? latestFinal.text : props.status.message}
            >
              {showTranscriptPeek && latestFinal ? (
                <>
                  <strong>{latestFinal.direction === "input" ? "You" : "Assistant"}</strong>
                  <span aria-hidden="true"> · </span>
                  {latestFinal.text}
                </>
              ) : props.status.message}
            </span>
          </span>
        </button>

        <div className="voice-secondary" aria-label="Voice controls" data-testid="dock-controls-cluster">
          <button
            type="button"
            className="dock-icon-button"
            aria-label={props.muted ? "Unmute microphone" : "Mute microphone"}
            title={`${props.muted ? "Unmute" : "Mute"} (Space)`}
            disabled={props.isMockMode || !props.started || props.starting}
            onClick={props.onMuteToggle}
          >
            {props.muted ? <MutedIcon /> : <MicIcon />}
          </button>
          <button
            type="button"
            className="dock-icon-button stop-button"
            aria-label="Stop voice session"
            title="Stop session"
            disabled={props.isMockMode || !props.started}
            onClick={props.onStop}
          >
            <StopIcon />
          </button>
        </div>

        <button
          ref={transcriptToggleRef}
          type="button"
          className="dock-icon-button transcript-toggle"
          aria-label={`${drawerOpen ? "Hide" : "Expand"} conversation`}
          aria-controls="transcript-content"
          aria-expanded={drawerOpen}
          title={`${drawerOpen ? "Hide" : "Expand"} conversation`}
          onClick={() => setDrawerOpen((open) => !open)}
        >
          <ChevronIcon />
        </button>
      </div>
    </section>
  );
}
