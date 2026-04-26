import { useEffect, useRef } from 'react';
import { storage } from '@/lib/storage';

export interface RealtimeEvent {
  topic: string;
  type: string;
  payload?: Record<string, unknown>;
}

/**
 * Subscribes to the backend WebSocket hub for the given topics and invokes
 * onEvent for every inbound message. The connection is torn down on unmount
 * or when dependencies change. A single reconnect is attempted with a small
 * backoff; this is intentionally simple since the app is mobile and the user
 * will notice a stale feed and refresh.
 */
export function useWebSocket(topics: string[], onEvent: (evt: RealtimeEvent) => void) {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    const token = storage.getAccess();
    if (!token || topics.length === 0) return;

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const params = new URLSearchParams();
    topics.forEach((t) => params.append('topic', t));
    // auth via subprotocol or query since browsers can't set headers on ws:
    params.set('access_token', token);

    const url = `${proto}://${window.location.host}/api/v1/ws?${params.toString()}`;

    let ws: WebSocket | null = null;
    let retry: number | null = null;
    let destroyed = false;

    const open = () => {
      const localWs = new WebSocket(url);
      ws = localWs;
      localWs.onmessage = (e) => {
        try {
          const evt = JSON.parse(e.data);
          handlerRef.current(evt);
        } catch {
          /* ignore bad frame */
        }
      };
      localWs.onclose = () => {
        if (destroyed) return;
        retry = window.setTimeout(() => {
          if (!destroyed) open();
        }, 3000);
      };
    };
    open();

    return () => {
      destroyed = true;
      if (retry !== null) {
        window.clearTimeout(retry);
        retry = null;
      }
      ws?.close();
      ws = null;
    };
  }, [topics.join(',')]); // eslint-disable-line react-hooks/exhaustive-deps
}
