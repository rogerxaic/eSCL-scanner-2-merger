// src/hooks/useWebSocket.ts
import { useState, useEffect } from 'react';
import { BroadcastData, ScanRequest, ScannerStatus, ScanResult } from '../types';

interface WebSocketState {
  ws: WebSocket | null;
  scanRequest: ScanRequest | null;
  scannerStatus: ScannerStatus | null;
  scanResults: { [key: string]: ScanResult };
}

export function useWebSocket({host, secure}: {host: string, secure: boolean}): [WebSocketState, React.Dispatch<React.SetStateAction<WebSocketState>>] {
  const [state, setState] = useState<WebSocketState>({
    ws: null,
    scanRequest: null,
    scannerStatus: null,
    scanResults: {}
  });

  useEffect(() => {
    const connectWebSocket = () => {
      const websocket = new WebSocket(`${secure ? "wss" : "ws"}://${host}/api/ws`);

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
            const res = { ...prev.scanResults };
            (data.scanResults ?? []).forEach((scanResult) => {
              res[scanResult.session] = scanResult;
            });
            newState.scanResults = res;
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
        setState(prev => ({ ...prev, ws: null }));
        setTimeout(connectWebSocket, 1500);
      };

      return websocket;
    };

    const websocket = connectWebSocket();
    return () => websocket.close();
  }, []);

  return [state, setState];
}