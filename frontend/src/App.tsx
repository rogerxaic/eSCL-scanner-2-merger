import React from "react";
import "./App.css";

import ScanResults from "./components/ScanResults";
import { InputSourceSelector } from "./components/InputSourceSelector";
import { ScannerStatus } from "./types";
import { BroadcastData } from "./types";
import { ScanRequest, ScanResult } from "./types";
import { ScanOptions } from "./components/ScanOptions";

function App() {
    const [ws, setWs] = React.useState<WebSocket | null>(null);

    const [scanRequest, setScanRequest] = React.useState<ScanRequest | null>(
        null
    );

    const [scannerStatus, setScannerStatus] =
        React.useState<ScannerStatus | null>(null);

    interface ScanResultsById {
        [key: string]: ScanResult;
    }
    const [scanResults, setScanResults] = React.useState<ScanResultsById>({});

    React.useEffect(() => {
        // Reconnect websocket if disconnected
        const connectWebSocket = () => {
            const websocket = new WebSocket(`ws://${window.location.host}/api/ws`);

            websocket.onopen = () => {
                console.log("Connected to websocket");
                setWs(websocket);
            };

            websocket.onmessage = (event) => {
                const data = JSON.parse(event.data) as BroadcastData;
                if (data.type === "scanRequest") {
                    setScanRequest({
                        ...data.scanRequest!,
                        recipients: data.scanRequest!.recipients ?? [],
                        inputSource: data.scanRequest!.inputSource || "Auto",
                    });
                } else if (data.type === "scanResult") {
                    const res = { ...scanResults };
                    (data.scanResults ?? []).forEach((scanResult) => {
                        res[scanResult.session] = scanResult;
                    });
                    setScanResults(res);
                } else if (data.type === "scannerStatus") {
                    setScannerStatus(data.scannerStatus!);
                }
            };

            websocket.onerror = (error) => {
                console.error("WebSocket error:", error);
            };

            websocket.onclose = () => {
                console.log("Disconnected from websocket");
                setWs(null);
                // Try to reconnect after 5 seconds
                setTimeout(connectWebSocket, 1500);
            };

            return websocket;
        };
        const websocket = connectWebSocket();

        return () => {
            websocket.close();
        };
    }, []);

    return (
        <div className="App">
            <div
                style={{
                    backgroundColor: ws
                        ? scannerStatus?.scanning
                            ? "#0066cc"
                            : "#00cc00"
                        : "#ff0000",
                    color: "white",
                    width: "100vw",
                    padding: "8px 0",
                    textAlign: "center",
                    position: "fixed",
                    top: 0,
                    left: 0,
                    margin: "0",
                    zIndex: 1000,
                }}
            >
                {ws
                    ? scannerStatus?.scanning
                        ? "Scanning..."
                        : "Connected to scanner"
                    : "Not connected to scanner"}
            </div>

            <div style={{ padding: "35px" }}>
                {scanRequest && (
                    <form className="scanner-form">
                        <InputSourceSelector
                            inputSource={scanRequest.inputSource}
                            onChange={(source) =>
                                setScanRequest({
                                    ...scanRequest,
                                    inputSource: source,
                                })
                            }
                        />

                        <hr />

                        <ScanOptions
                            scanRequest={scanRequest}
                            onUpdate={setScanRequest}
                        />
                    </form>
                )}

                {/* <pre>{JSON.stringify(scanRequest, null, 2)}</pre> */}

                <hr style={{ width: "100%", margin: "20px 0" }} />
                {scanRequest && (
                    <button
                        onClick={async () => {
                            try {
                                const response = await fetch(
                                    `${window.location.protocol}//${window.location.host}/api/scan`,
                                    {
                                        method: "POST",
                                        headers: {
                                            "Content-Type": "application/json",
                                        },
                                        body: JSON.stringify(scanRequest),
                                    }
                                );

                                if (!response.ok) {
                                    throw new Error(
                                        `HTTP error! status: ${response.status}`
                                    );
                                }

                                console.log("Scan initiated successfully");
                            } catch (error) {
                                console.error("Error initiating scan:", error);
                            }
                        }}
                        disabled={scannerStatus?.scanning}
                    >
                        Scan
                    </button>
                )}
                <hr style={{ width: "100%", margin: "20px 0" }} />
                <ScanResults results={scanResults} />
            </div>
        </div>
    );
}

export default App;
