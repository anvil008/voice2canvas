import { parseServerFrame, type ClientFrame, type ServerFrame } from "./protocol";

export type ConnectionState = "idle" | "connecting" | "connected" | "reconnecting" | "closed";

interface WsEvents {
  binary: ArrayBuffer;
  connection: ConnectionState;
  frame: ServerFrame;
  protocol_error: { message: string; raw?: string };
  reconnecting: { attempt: number; delayMs: number };
  socket_error: Event;
}

type Listener<T> = (event: T) => void;

/** A small typed event emitter so the React layer does not own socket callbacks. */
class TypedEmitter<Events extends object> {
  private readonly listeners = new Map<keyof Events, Set<Listener<never>>>();

  on<Key extends keyof Events>(event: Key, listener: Listener<Events[Key]>): () => void {
    const eventListeners = this.listeners.get(event) ?? new Set<Listener<never>>();
    eventListeners.add(listener as Listener<never>);
    this.listeners.set(event, eventListeners);

    return () => {
      eventListeners.delete(listener as Listener<never>);
      if (eventListeners.size === 0) {
        this.listeners.delete(event);
      }
    };
  }

  protected emit<Key extends keyof Events>(event: Key, payload: Events[Key]): void {
    for (const listener of this.listeners.get(event) ?? []) {
      (listener as Listener<Events[Key]>)(payload);
    }
  }

  protected clearListeners(): void {
    this.listeners.clear();
  }
}

function defaultWebSocketUrl(): string {
  const url = new URL("/ws", window.location.href);
  url.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

export class WsClient extends TypedEmitter<WsEvents> {
  private socket: WebSocket | undefined;
  private reconnectTimer: number | undefined;
  private reconnectAttempt = 0;
  private shouldReconnect = false;
  private _state: ConnectionState = "idle";

  constructor(private readonly url = defaultWebSocketUrl()) {
    super();
  }

  get state(): ConnectionState {
    return this._state;
  }

  get connected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }

  connect(): void {
    this.shouldReconnect = true;
    if (
      this.socket?.readyState === WebSocket.OPEN ||
      this.socket?.readyState === WebSocket.CONNECTING
    ) {
      return;
    }
    this.openSocket(false);
  }

  disconnect(): void {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    const socket = this.socket;
    this.socket = undefined;
    socket?.close();
    this.setState("closed");
  }

  dispose(): void {
    this.disconnect();
    this.clearListeners();
  }

  send(frame: ClientFrame): boolean {
    if (!this.connected || !this.socket) {
      return false;
    }
    this.socket.send(JSON.stringify(frame));
    return true;
  }

  sendAudio(pcm: ArrayBuffer): boolean {
    if (!this.connected || !this.socket) {
      return false;
    }
    this.socket.send(pcm);
    return true;
  }

  private openSocket(isReconnect: boolean): void {
    this.clearReconnectTimer();
    this.setState(isReconnect ? "reconnecting" : "connecting");

    let socket: WebSocket;
    try {
      socket = new WebSocket(this.url);
    } catch (error) {
      this.emit("protocol_error", {
        message: error instanceof Error ? error.message : "Could not create WebSocket.",
      });
      this.scheduleReconnect();
      return;
    }

    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.onopen = () => {
      if (this.socket !== socket) {
        return;
      }
      this.reconnectAttempt = 0;
      this.setState("connected");
    };

    socket.onmessage = (event) => {
      if (this.socket !== socket) {
        return;
      }
      void this.handleMessage(event.data);
    };

    socket.onerror = (event) => {
      if (this.socket === socket) {
        this.emit("socket_error", event);
      }
    };

    socket.onclose = () => {
      if (this.socket !== socket) {
        return;
      }
      this.socket = undefined;
      if (this.shouldReconnect) {
        this.scheduleReconnect();
      } else {
        this.setState("closed");
      }
    };
  }

  private async handleMessage(data: unknown): Promise<void> {
    if (data instanceof ArrayBuffer) {
      this.emit("binary", data);
      return;
    }

    if (data instanceof Blob) {
      this.emit("binary", await data.arrayBuffer());
      return;
    }

    if (typeof data !== "string") {
      this.emit("protocol_error", { message: "Received an unsupported WebSocket frame." });
      return;
    }

    let json: unknown;
    try {
      json = JSON.parse(data);
    } catch {
      this.emit("protocol_error", { message: "Received invalid JSON from the server.", raw: data });
      return;
    }

    const frame = parseServerFrame(json);
    if (!frame) {
      this.emit("protocol_error", { message: "Received an invalid protocol frame.", raw: data });
      return;
    }
    this.emit("frame", frame);
  }

  private scheduleReconnect(): void {
    if (!this.shouldReconnect || this.reconnectTimer !== undefined) {
      return;
    }

    const attempt = ++this.reconnectAttempt;
    const baseDelay = Math.min(10_000, 500 * 2 ** (attempt - 1));
    const delayMs = Math.round(baseDelay * (0.8 + Math.random() * 0.4));
    this.setState("reconnecting");
    this.emit("reconnecting", { attempt, delayMs });
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = undefined;
      this.openSocket(true);
    }, delayMs);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== undefined) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = undefined;
    }
  }

  private setState(state: ConnectionState): void {
    if (this._state === state) {
      return;
    }
    this._state = state;
    this.emit("connection", state);
  }
}
