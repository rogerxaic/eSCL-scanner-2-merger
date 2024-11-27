import React from "react";
import "./App.css";

import ScanResults from "./components/ScanResults";
import { InputSourceSelector } from "./components/InputSourceSelector";
import { ScanOptions } from "./components/ScanOptions";
import { useWebSocket } from "./hooks/useWebSocket";
import { initiateScan } from "./utils/scanUtils";
import { StatusBar } from "./components/StatusBar";

function App(): React.ReactElement {
    const host = window.location.host;
    const secure = window.location.protocol === "https:";
    const apiBaseUrl = `${secure ? "https" : "http"}://${host}`;

    const [{ ws, scanRequest, scannerStatus, scanResults }, setWebSocketState] =
        useWebSocket({ host, secure });

    return (
        <div className="App">
            <StatusBar ws={ws} scannerStatus={scannerStatus} />

            <div className="content-wrapper">
                {scanRequest && (
                    <form className="scanner-form">
                        <div className="scanner-form__row">
                            <InputSourceSelector
                                onScan={(inputSource) =>
                                    initiateScan(inputSource, apiBaseUrl)
                                }
                                disabled={scannerStatus?.scanning || !ws}
                            />
                        </div>
                        <hr />
                        <ScanOptions
                            scanRequest={scanRequest}
                            onUpdate={(scanRequest) =>
                                setWebSocketState((prev) => ({
                                    ...prev,
                                    scanRequest,
                                }))
                            }
                        />
                    </form>
                )}

                <hr className="divider" />
                <ScanResults results={scanResults} apiBaseUrl={apiBaseUrl} />
            </div>
        </div>
    );
}

export default App;
