import type { WsClientOptions, WsEnvelope, WsEventHandler } from "../types/ws";

type ConnectionStatus = "idle" | "connecting" | "open" | "closed";

export type ConnectionStatusHandler = (status: ConnectionStatus) => void;

type WebSocketFactory = new (url: string) => WebSocket;

const toWsUrl = (baseUrl: string, token?: string | null): string => {
  const url = (() => {
    if (/^wss?:\/\//.test(baseUrl) || /^https?:\/\//.test(baseUrl)) {
      return new URL(baseUrl);
    }
    if (typeof window !== "undefined" && window.location?.origin) {
      return new URL(baseUrl, window.location.origin);
    }
    return new URL(baseUrl, "http://localhost");
  })();
  if (url.protocol === "http:") {
    url.protocol = "ws:";
  } else if (url.protocol === "https:") {
    url.protocol = "wss:";
  }

  const normalizedPath = url.pathname.replace(/\/+$/, "");
  url.pathname = normalizedPath.endsWith("/ws")
    ? normalizedPath
    : `${normalizedPath}/ws`;

  if (token) {
    url.searchParams.set("token", token);
  } else {
    url.searchParams.delete("token");
  }

  return url.toString();
};

export class WsClient {
  private readonly baseUrl: string;
  private readonly getToken?: () => string | null | undefined;
  private readonly reconnectIntervalMs: number;
  private readonly maxReconnectIntervalMs: number;
  private readonly wsFactory: WebSocketFactory;
  private socket: WebSocket | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private manuallyClosed = false;
  private status: ConnectionStatus = "idle";
  private readonly listeners = new Map<string, Set<WsEventHandler>>();
  private readonly statusListeners = new Set<ConnectionStatusHandler>();

  constructor(options: WsClientOptions, wsFactory?: WebSocketFactory) {
    this.baseUrl = options.baseUrl;
    this.getToken = options.getToken;
    this.reconnectIntervalMs = options.reconnectIntervalMs ?? 1000;
    this.maxReconnectIntervalMs = options.maxReconnectIntervalMs ?? 10000;
    this.wsFactory = wsFactory ?? WebSocket;
  }

  connect(): void {
    if (
      this.socket &&
      (this.socket.readyState === WebSocket.OPEN ||
        this.socket.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }
    this.manuallyClosed = false;
    this.updateStatus("connecting");
    this.openSocket();
  }

  disconnect(code?: number, reason?: string): void {
    this.manuallyClosed = true;
    this.clearReconnectTimer();
    if (this.socket) {
      this.socket.close(code, reason);
      this.socket = null;
    }
    this.updateStatus("closed");
  }

  send(data: WsEnvelope | string): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error("WebSocket is not connected");
    }
    const payload = typeof data === "string" ? data : JSON.stringify(data);
    this.socket.send(payload);
  }

  subscribe<TPayload = unknown>(
    type: string,
    handler: WsEventHandler<TPayload>,
  ): () => void {
    const setForType = this.listeners.get(type) ?? new Set<WsEventHandler>();
    setForType.add(handler as WsEventHandler);
    this.listeners.set(type, setForType);

    return () => {
      const current = this.listeners.get(type);
      if (!current) {
        return;
      }
      current.delete(handler as WsEventHandler);
      if (current.size === 0) {
        this.listeners.delete(type);
      }
    };
  }

  onStatusChange(handler: ConnectionStatusHandler): () => void {
    this.statusListeners.add(handler);
    return () => {
      this.statusListeners.delete(handler);
    };
  }

  getStatus(): ConnectionStatus {
    return this.status;
  }

  private openSocket(): void {
    const token = this.getToken?.();
    const socket = new this.wsFactory(toWsUrl(this.baseUrl, token));
    this.socket = socket;

    socket.onopen = () => {
      this.reconnectAttempt = 0;
      this.clearReconnectTimer();
      this.updateStatus("open");
    };

    socket.onmessage = (event: MessageEvent<string>) => {
      this.routeMessage(event);
    };

    socket.onclose = () => {
      this.socket = null;
      this.updateStatus("closed");
      if (!this.manuallyClosed) {
        this.scheduleReconnect();
      }
    };

    socket.onerror = () => {
      if (!this.manuallyClosed) {
        this.scheduleReconnect();
      }
    };
  }

  private routeMessage(event: MessageEvent<string>): void {
    if (typeof event.data !== "string") {
      return;
    }

    let envelope: WsEnvelope | null;
    try {
      envelope = JSON.parse(event.data) as WsEnvelope;
    } catch {
      return;
    }

    if (envelope && typeof envelope.type === "string") {
      this.emit(
        envelope.type,
        envelope.data !== undefined ? envelope.data : envelope.payload,
        event,
      );
      this.emit("*", envelope, event);
      return;
    }

    this.emit("raw", event.data, event);
  }

  private emit(type: string, payload: unknown, event: MessageEvent<string>): void {
    const handlers = this.listeners.get(type);
    if (!handlers || handlers.size === 0) {
      return;
    }
    handlers.forEach((handler) => {
      handler(payload, event);
    });
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer || this.manuallyClosed) {
      return;
    }

    const delay = Math.min(
      this.reconnectIntervalMs * 2 ** this.reconnectAttempt,
      this.maxReconnectIntervalMs,
    );
    this.reconnectAttempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.manuallyClosed) {
        return;
      }
      this.updateStatus("connecting");
      this.openSocket();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (!this.reconnectTimer) {
      return;
    }
    clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }

  private updateStatus(nextStatus: ConnectionStatus): void {
    if (this.status === nextStatus) {
      return;
    }
    this.status = nextStatus;
    this.statusListeners.forEach((handler) => {
      handler(nextStatus);
    });
  }
}

export const createWsClient = (options: WsClientOptions): WsClient =>
  new WsClient(options);
