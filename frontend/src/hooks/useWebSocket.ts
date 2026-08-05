// src/hooks/useWebSocket.ts
import { useState, useEffect } from 'react';
import { BroadcastData, ScanRequest, ScannerStatus, ScanResult } from '../types';

interface WebSocketState {
  ws: WebSocket | null;
  scanRequest: ScanRequest | null;
  scannerStatus: ScannerStatus | null;
  scanResults: { [key: string]: ScanResult };
}

export function useWebSocket({host, secure}: {host: string, secure: boolean}): WebSocketState {
  const [state, setState] = useState<WebSocketState>({
    ws: null,
    scanRequest: null,
    scannerStatus: null,
    scanResults: {}
  });

  useEffect(() => {
    let cancelled = false;
    let websocket: WebSocket;

    const connectWebSocket = () => {
      websocket = new WebSocket(`${secure ? "wss" : "ws"}://${host}/api/ws`);

      websocket.onopen = () => {
        console.log("Connected to websocket");
        setState(prev => ({ ...prev, ws: websocket }));
      };

      websocket.onmessage = (event) => {
        const data = JSON.parse(event.data) as BroadcastData;
        setState(prev => {
          const newState = { ...prev };

          if (data.type === "scanRequest" && data.scanRequest) {
            newState.scanRequest = {
              ...data.scanRequest,
              recipients: data.scanRequest.recipients ?? [],
              inputSource: data.scanRequest.inputSource || "Auto",
            };
          } else if (data.type === "scanResult") {
            // The server always sends the complete list, so replace the
            // map wholesale — merging would keep deleted sessions forever.
            newState.scanResults = Object.fromEntries(
              (data.scanResults ?? []).map((r) => [r.session, r])
            );
          } else if (data.type === "scannerStatus") {
            newState.scannerStatus = data.scannerStatus!;
          }

          return newState;
        });
      };

      websocket.onerror = (error) => {
        console.error("WebSocket error:", error);
      };

      websocket.onclose = () => {
        console.log("Disconnected from websocket");
        if (cancelled) return;
        setState(prev => ({ ...prev, ws: null }));
        setTimeout(() => {
          if (!cancelled) connectWebSocket();
        }, 1500);
      };
    };

    connectWebSocket();
    return () => {
      cancelled = true;
      websocket.close();
    };
  }, [host, secure]);

  return state;
}
