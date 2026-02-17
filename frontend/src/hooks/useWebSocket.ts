"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import type { WSMessage } from "@/lib/api";

interface UseWebSocketOptions {
  url: string | null;
  onMessage?: (msg: WSMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export function useWebSocket({ url, onMessage, onOpen, onClose }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

  const connect = useCallback(() => {
    if (!url) return;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      onOpen?.();
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage;
        onMessage?.(msg);
      } catch {
        // ignore invalid messages
      }
    };

    ws.onclose = () => {
      setConnected(false);
      onClose?.();
      // Auto-reconnect after 3s
      setTimeout(() => connect(), 3000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [url, onMessage, onOpen, onClose]);

  useEffect(() => {
    connect();
    return () => {
      wsRef.current?.close();
    };
  }, [connect]);

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  }, []);

  return { connected, send };
}
