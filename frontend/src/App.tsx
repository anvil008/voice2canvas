import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { A2uiCanvas, A2uiRuntime } from "./a2ui/renderer";
import { MicCapture } from "./audio/mic";
import { PcmPlayback } from "./audio/playback";
import { runMockScript } from "./mock";
import type { ServerFrame } from "./protocol";
import { dashboardReducer, initialDashboardState } from "./reducer";
import { TaskToasts } from "./ui/TaskToasts";
import { ThemeToggle, type Theme } from "./ui/ThemeToggle";
import { AgentsPage } from "./ui/AgentsPage";
import { AgentsIcon, ConnectIcon } from "./ui/icons";
import { VoiceDock } from "./ui/VoiceDock";
import { WsClient, type ConnectionState } from "./ws";
import { ApiKeyDialog } from "./ui/ApiKeyDialog";

type HealthState = "checking" | "healthy" | "unavailable" | "mock";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function mockModeEnabled(): boolean {
  return new URLSearchParams(window.location.search).get("mock") === "1";
}

function initialTheme(): Theme {
  const documentTheme = document.documentElement.dataset.theme;
  if (documentTheme === "light" || documentTheme === "dark") return documentTheme;
  try {
    const saved = window.localStorage.getItem("voice2canvas_theme") || window.localStorage.getItem("peitho_theme");
    if (saved === "light" || saved === "dark") return saved;
  } catch {}
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === "INPUT" ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT"
  );
}

export default function App() {
  const isMockMode = mockModeEnabled();
  const [dashboard, dispatch] = useReducer(dashboardReducer, initialDashboardState);
  const [connection, setConnection] = useState<ConnectionState>(
    isMockMode ? "connected" : "connecting",
  );
  const [health, setHealth] = useState<HealthState>(isMockMode ? "mock" : "checking");
  const [muted, setMuted] = useState(true);
  const [started, setStarted] = useState(false);
  const [starting, setStarting] = useState(false);
  const [theme, setTheme] = useState<Theme>(initialTheme);
  const [agentsOpen, setAgentsOpen] = useState(false);
  const [apiKeyOpen, setApiKeyOpen] = useState(false);
  const [apiKey, setApiKey] = useState("");
  const [serverAuthConfigured, setServerAuthConfigured] = useState<boolean>();

  const clientRef = useRef<WsClient | undefined>(undefined);
  const micRef = useRef<MicCapture | undefined>(undefined);
  const playbackRef = useRef<PcmPlayback | undefined>(undefined);
  const startAttemptRef = useRef(0);
  const wantsSessionRef = useRef(false);
  const canSendAudioRef = useRef(false);
  const agentsButtonRef = useRef<HTMLButtonElement>(null);
  const startAfterKeyRef = useRef(false);
  const apiKeyRef = useRef("");

  const [a2uiRuntime] = useState(
    () =>
      new A2uiRuntime((action) => {
        const sent = clientRef.current?.send({ type: "action", ...action }) ?? false;
        if (!sent) {
          dispatch({
            type: "local_status",
            message: `Could not send action “${action.name}”: socket is not connected.`,
            tone: "error",
          });
        }
      }),
  );

  const getMic = (): MicCapture => {
    micRef.current ??= new MicCapture();
    return micRef.current;
  };

  const getPlayback = (): PcmPlayback => {
    playbackRef.current ??= new PcmPlayback();
    return playbackRef.current;
  };

  const handleFrame = useCallback(
    (frame: ServerFrame) => {
      if (frame.type === "a2ui") {
        try {
          a2uiRuntime.feed(frame.messages);
        } catch (error) {
          dispatch({
            type: "local_status",
            message: `A2UI render error: ${errorMessage(error)}`,
            tone: "error",
          });
          return;
        }
      }

      if (frame.type === "ready") {
        a2uiRuntime.reset();
        canSendAudioRef.current = true;
        setStarted(true);
        setStarting(false);
      }
      if (frame.type === "interrupted") {
        playbackRef.current?.flush();
      }
      if (frame.type === "error" && frame.fatal) {
        startAttemptRef.current += 1;
        canSendAudioRef.current = false;
        wantsSessionRef.current = false;
        micRef.current?.stop();
        playbackRef.current?.flush();
        setMuted(true);
        setStarted(false);
        setStarting(false);
      }

      dispatch({ type: "server", frame });
    },
    [a2uiRuntime],
  );

  useEffect(() => {
    if (isMockMode) {
      dispatch({
        type: "local_status",
        message: "Mock mode: playing canned protocol frames",
        tone: "working",
      });
      return runMockScript(handleFrame);
    }

    const client = new WsClient();
    clientRef.current = client;
    const unsubscribers = [
      client.on("frame", handleFrame),
      client.on("binary", (pcm) => {
        const player = playbackRef.current;
        if (player) {
          void player.queue(pcm).catch((error: unknown) => {
            dispatch({
              type: "local_status",
              message: `Playback error: ${errorMessage(error)}`,
              tone: "error",
            });
          });
        }
      }),
      client.on("connection", (state) => {
        setConnection(state);
        if (state === "connected" && wantsSessionRef.current) {
          canSendAudioRef.current = false;
          client.send({ type: "start", apiKey: apiKeyRef.current || undefined });
        }
      }),
      client.on("reconnecting", ({ attempt, delayMs }) => {
        dispatch({
          type: "local_status",
          message: `Reconnecting (attempt ${attempt}) in ${delayMs} ms…`,
          tone: "working",
        });
      }),
      client.on("protocol_error", ({ message }) => {
        dispatch({ type: "local_status", message: `Protocol error: ${message}`, tone: "error" });
      }),
      client.on("socket_error", () => {
        dispatch({ type: "local_status", message: "WebSocket transport error", tone: "error" });
      }),
    ];
    client.connect();

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe());
      client.dispose();
      if (clientRef.current === client) {
        clientRef.current = undefined;
      }
    };
  }, [handleFrame, isMockMode]);

  useEffect(() => {
    if (isMockMode) {
      setHealth("mock");
      return;
    }

    const controller = new AbortController();
    void fetch("/healthz", { signal: controller.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }
        return response.json() as Promise<{ authConfigured?: boolean }>;
      })
      .then((body) => {
        setServerAuthConfigured(body.authConfigured === true);
        setHealth("healthy");
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setHealth("unavailable");
          dispatch({
            type: "local_status",
            message: `Backend health check failed: ${errorMessage(error)}`,
            tone: "error",
          });
        }
      });

    return () => controller.abort();
  }, [isMockMode]);

  useEffect(
    () => () => {
      void micRef.current?.close();
      void playbackRef.current?.close();
    },
    [],
  );

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute("content", theme === "light" ? "#FFFFFF" : "#131314");
    try {
      window.localStorage.setItem("voice2canvas_theme", theme);
    } catch {
      // Storage can be unavailable in hardened/private browser contexts.
    }
  }, [theme]);


  const sendMicChunk = useCallback((pcm16Le: ArrayBuffer) => {
    if (canSendAudioRef.current) {
      clientRef.current?.sendAudio(pcm16Le);
    }
  }, []);

  const startSession = async (): Promise<void> => {
    if (isMockMode) {
      dispatch({
        type: "local_status",
        message: "Mock mode is already playing; reload ?mock=1 to replay it.",
        tone: "working",
      });
      return;
    }

    if (!apiKey && serverAuthConfigured === false) {
      startAfterKeyRef.current = true;
      setApiKeyOpen(true);
      return;
    }

    const attempt = ++startAttemptRef.current;
    wantsSessionRef.current = true;
    canSendAudioRef.current = false;
    setMuted(false);
    setStarted(true);
    setStarting(true);
    dispatch({ type: "local_status", message: "Starting microphone…", tone: "working" });

    const client = clientRef.current;
    client?.connect();
    client?.send({ type: "start", apiKey: apiKey || undefined });

    try {
      await getPlayback().activate();
      if (!wantsSessionRef.current || attempt !== startAttemptRef.current) {
        return;
      }
      await getMic().start(sendMicChunk);
      if (!wantsSessionRef.current || attempt !== startAttemptRef.current) {
        getMic().stop();
      }
    } catch (error) {
      if (attempt !== startAttemptRef.current) {
        return;
      }
      wantsSessionRef.current = false;
      canSendAudioRef.current = false;
      setMuted(true);
      setStarted(false);
      setStarting(false);
      clientRef.current?.send({ type: "audio_end" });
      dispatch({
        type: "local_status",
        message: `Microphone error: ${errorMessage(error)}`,
        tone: "error",
      });
    }
  };

  const mute = (): void => {
    getMic().stop();
    clientRef.current?.send({ type: "audio_end" });
    setMuted(true);
    dispatch({ type: "local_status", message: "Microphone muted", tone: "working" });
  };

  const unmute = async (): Promise<void> => {
    if (isMockMode || !started) {
      return;
    }
    try {
      await getPlayback().activate();
      await getMic().start(sendMicChunk);
      setMuted(false);
      dispatch({ type: "local_status", message: "Microphone active", tone: "working" });
    } catch (error) {
      setMuted(true);
      dispatch({
        type: "local_status",
        message: `Microphone error: ${errorMessage(error)}`,
        tone: "error",
      });
    }
  };

  const stopSession = (): void => {
    startAttemptRef.current += 1;
    canSendAudioRef.current = false;
    wantsSessionRef.current = false;
    micRef.current?.stop();
    playbackRef.current?.flush();
    clientRef.current?.send({ type: "stop" });
    setMuted(true);
    setStarted(false);
    setStarting(false);
    dispatch({ type: "local_status", message: "Session stopped", tone: "idle" });
  };

  const toggleMute = useCallback((): void => {
    if (isMockMode || !started || starting) return;
    if (muted) void unmute();
    else mute();
  }, [isMockMode, muted, started, starting]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.code !== "Space" || event.repeat || isTypingTarget(event.target)) return;
      if (isMockMode || !started || starting) return;
      event.preventDefault();
      toggleMute();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isMockMode, started, starting, toggleMute]);

  const mainVoiceAction = (): void => {
    if (!started) {
      void startSession();
      return;
    }
    toggleMute();
  };

  const pendingCards = useMemo(
    () => dashboard.pendingCards.map((pending) => ({
      ...pending,
      status: dashboard.tasks[pending.taskId]?.status,
    })),
    [dashboard.pendingCards, dashboard.tasks],
  );

  const closeAgents = useCallback(() => {
    setAgentsOpen(false);
    agentsButtonRef.current?.focus();
  }, []);

  const saveApiKey = (value: string) => {
    apiKeyRef.current = value;
    setApiKey(value);
    setApiKeyOpen(false);
    dispatch({ type: "local_status", message: "Gemini key ready for this tab", tone: "idle" });
    if (startAfterKeyRef.current) {
      startAfterKeyRef.current = false;
      queueMicrotask(() => void startSessionWithKey(value));
    }
  };

  const startSessionWithKey = async (value: string): Promise<void> => {
    const attempt = ++startAttemptRef.current;
    wantsSessionRef.current = true;
    canSendAudioRef.current = false;
    setMuted(false);
    setStarted(true);
    setStarting(true);
    dispatch({ type: "local_status", message: "Starting microphone…", tone: "working" });
    clientRef.current?.connect();
    clientRef.current?.send({ type: "start", apiKey: value });
    try {
      await getPlayback().activate();
      if (!wantsSessionRef.current || attempt !== startAttemptRef.current) return;
      await getMic().start(sendMicChunk);
    } catch (error) {
      if (attempt !== startAttemptRef.current) return;
      wantsSessionRef.current = false;
      setMuted(true);
      setStarted(false);
      setStarting(false);
      dispatch({ type: "local_status", message: `Microphone error: ${errorMessage(error)}`, tone: "error" });
    }
  };

  return (
    <>
      <main className="dashboard-shell" aria-hidden={agentsOpen || apiKeyOpen || undefined}>
        <header className="corner-header">
          <div className="wordmark" aria-label="Voice2Canvas">
            <span className="wordmark-mark" aria-hidden="true"><i /><i /><i /></span>
            <span>Voice2Canvas</span>
          </div>
          {isMockMode ? <span className="mock-badge">Mock mode</span> : null}
        </header>

        <div className="corner-actions">
          <button
            aria-label={apiKey ? "Gemini connected" : "Connect Gemini"}
            className={`theme-toggle api-key-toggle${apiKey ? " is-connected" : ""}`}
            onClick={() => setApiKeyOpen(true)}
            title={apiKey ? "Gemini connected (click to replace key)" : "Connect Gemini"}
            type="button"
          >
            <ConnectIcon />
          </button>
          <button
            className="theme-toggle agents-toggle"
            type="button"
            aria-label="Agents"
            title="Agents"
            onClick={() => setAgentsOpen(true)}
            ref={agentsButtonRef}
          >
            <AgentsIcon />
          </button>
          <ThemeToggle
            theme={theme}
            onToggle={() => setTheme((current) => (current === "light" ? "dark" : "light"))}
          />
        </div>

        <TaskToasts tasks={dashboard.tasks} />

        <section className="card-canvas" aria-label="Dashboard cards">
          <A2uiCanvas
            key={dashboard.canvasGeneration}
            pendingCards={pendingCards}
            runtime={a2uiRuntime}
            slots={dashboard.slots}
          />
        </section>

        <div className="voice-layer">
          <VoiceDock
            connection={connection}
            health={health}
            isMockMode={isMockMode}
            muted={muted}
            started={started}
            starting={starting}
            status={dashboard.status}
            transcripts={dashboard.transcripts}
            onMainAction={mainVoiceAction}
            onMuteToggle={toggleMute}
            onStop={stopSession}
          />
        </div>
      </main>
      {agentsOpen ? <AgentsPage onClose={closeAgents} /> : null}
      {apiKeyOpen ? <ApiKeyDialog onCancel={() => { startAfterKeyRef.current = false; setApiKeyOpen(false); }} onSave={saveApiKey} /> : null}
    </>
  );
}
