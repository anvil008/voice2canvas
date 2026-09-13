import { useCallback, useEffect, useRef, useState } from "react";
import { CloseIcon, VoiceBackIcon } from "./icons";

type AgentKind = "live" | "llm" | "system";

interface AgentTool {
  name: string;
  description: string;
}

interface AgentDataSource {
  name: string;
  detail: string;
  configured: boolean;
}

interface Agent {
  id: string;
  name: string;
  kind: AgentKind;
  model?: string;
  description: string;
  tools: AgentTool[];
  dataSources: AgentDataSource[];
  voiceBack?: boolean;
}

type RosterState =
  | { status: "loading" }
  | { status: "ready"; agents: Agent[] }
  | { status: "error" };

const focusableSelector = [
  "button:not([disabled])",
  "[href]",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",");

function kindLabel(kind: AgentKind): string {
  return kind === "llm" ? "worker" : kind;
}

function LoadingCards() {
  return (
    <div className="agents-grid agents-grid--loading" aria-label="Loading agents" aria-busy="true">
      {[0, 1, 2].map((index) => (
        <article className="agent-card agent-card--skeleton" key={index} aria-hidden="true">
          <span className="agent-skeleton agent-skeleton--title" />
          <span className="agent-skeleton agent-skeleton--copy" />
          <span className="agent-skeleton agent-skeleton--copy-short" />
          <span className="agent-skeleton agent-skeleton--section" />
          <span className="agent-skeleton agent-skeleton--rows" />
        </article>
      ))}
    </div>
  );
}

function AgentCard({ agent }: { agent: Agent }) {
  const displayedKind = kindLabel(agent.kind);

  return (
    <article className="agent-card">
      <header className="agent-card__header">
        <div className="agent-card__identity">
          <div className="agent-card__name-row">
            <h2>{agent.name}</h2>
            {agent.voiceBack ? (
              <span
                className="agent-voice-back"
                aria-label="Results are spoken through the live session"
                title="Results are spoken through the live session"
              >
                <VoiceBackIcon />
              </span>
            ) : null}
          </div>
          <div className="agent-card__chips">
            <span className="agent-kind" data-kind={displayedKind}>{displayedKind}</span>
            {agent.model ? <span className="agent-model">{agent.model}</span> : null}
          </div>
        </div>
        <p>{agent.description}</p>
      </header>

      <section className="agent-card__section" aria-labelledby={`${agent.id}-tools`}>
        <h3 id={`${agent.id}-tools`}>Tools</h3>
        {agent.tools.length > 0 ? (
          <div className="agent-tools">
            {agent.tools.map((tool) => (
              <span className="agent-tool" key={tool.name} title={tool.description}>
                {tool.name}
              </span>
            ))}
          </div>
        ) : <p className="agent-card__empty">No tools assigned</p>}
      </section>

      <section className="agent-card__section" aria-labelledby={`${agent.id}-data`}>
        <h3 id={`${agent.id}-data`}>Data access</h3>
        {agent.dataSources.length > 0 ? (
          <ul className="agent-data-sources">
            {agent.dataSources.map((source) => {
              const status = source.configured ? "Configured" : "Not configured";
              return (
                <li key={source.name}>
                  <span
                    className="agent-source-dot"
                    data-configured={source.configured}
                    aria-label={`${source.name}: ${status}`}
                    title={status}
                  />
                  <span className="agent-source-copy">
                    <strong>{source.name}</strong>
                    <small>{source.detail}</small>
                  </span>
                </li>
              );
            })}
          </ul>
        ) : <p className="agent-card__empty">No data access</p>}
      </section>
    </article>
  );
}

export function AgentsPage({ onClose }: { onClose: () => void }) {
  const [roster, setRoster] = useState<RosterState>({ status: "loading" });
  const [requestKey, setRequestKey] = useState(0);
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    document.body.classList.add("agents-view-open");
    closeRef.current?.focus();
    return () => document.body.classList.remove("agents-view-open");
  }, []);

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      onClose();
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [onClose]);

  useEffect(() => {
    const controller = new AbortController();
    setRoster({ status: "loading" });

    void fetch("/api/agents", { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const payload = await response.json() as { agents?: Agent[] };
        if (!Array.isArray(payload.agents)) throw new Error("Invalid agents response");
        setRoster({ status: "ready", agents: payload.agents });
      })
      .catch(() => {
        if (!controller.signal.aborted) setRoster({ status: "error" });
      });

    return () => controller.abort();
  }, [requestKey]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Tab") return;
    const focusable = Array.from(
      dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [],
    );
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }, []);

  return (
    <div
      className="agents-view"
      role="dialog"
      aria-modal="true"
      aria-labelledby="agents-title"
      aria-describedby="agents-subtitle"
      ref={dialogRef}
      onKeyDown={handleKeyDown}
    >
      <header className="agents-view__header">
        <div>
          <h1 id="agents-title">Agents</h1>
          <p id="agents-subtitle">General-purpose roles for weather, markets, research, and interface generation.</p>
        </div>
        <button
          className="agents-close"
          type="button"
          aria-label="Close Agents"
          title="Close Agents"
          onClick={onClose}
          ref={closeRef}
        >
          <CloseIcon />
        </button>
      </header>

      <div className="agents-view__body">
        {roster.status === "loading" ? <LoadingCards /> : null}
        {roster.status === "error" ? (
          <div className="agents-error" role="alert">
            <h2>Couldn’t reach the agent roster</h2>
            <p>The backend may be offline or unavailable in mock mode.</p>
            <button type="button" onClick={() => setRequestKey((key) => key + 1)}>Try again</button>
          </div>
        ) : null}
        {roster.status === "ready" ? (
          roster.agents.length > 0 ? (
            <div className="agents-grid">
              {roster.agents.map((agent) => <AgentCard agent={agent} key={agent.id} />)}
            </div>
          ) : (
            <div className="agents-empty"><p>No agents are registered yet.</p></div>
          )
        ) : null}
      </div>
    </div>
  );
}
