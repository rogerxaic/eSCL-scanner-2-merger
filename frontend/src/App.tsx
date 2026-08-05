import React from "react";
import "./App.css";

import ScanResults from "./components/ScanResults";
import { InputSourceSelector } from "./components/InputSourceSelector";
import { useWebSocket } from "./hooks/useWebSocket";
import { initiateScan } from "./utils/scanUtils";
import { StatusBar } from "./components/StatusBar";

function App(): React.ReactElement {
    const host = window.location.host;
    const secure = window.location.protocol === "https:";
    const apiBaseUrl = `${secure ? "https" : "http"}://${host}`;

    const { ws, scanRequest, scannerStatus, scanResults } =
        useWebSocket({ host, secure });

    const [scanError, setScanError] = React.useState<string | null>(null);

    const handleScan = async (inputSource: Parameters<typeof initiateScan>[0]) => {
        setScanError(null);
        const ok = await initiateScan(inputSource, apiBaseUrl);
        if (!ok) {
            setScanError("Failed to start scan — is the scanner on?");
        }
    };

    return (
        <div className="App">
            <StatusBar ws={ws} scannerStatus={scannerStatus} />

            <div className="content-wrapper">
                {scanRequest && (
                    <form className="scanner-form" onSubmit={(e) => e.preventDefault()}>
                        <div className="scanner-form__row">
                            <InputSourceSelector
                                onScan={handleScan}
                                disabled={scannerStatus?.scanning || !ws}
                            />
                        </div>
                        {scanError && (
                            <p className="scan-error">{scanError}</p>
                        )}
                        <hr />
                    </form>
                )}

                <ScanResults results={scanResults} apiBaseUrl={apiBaseUrl} />
            </div>
        </div>
    );
}

export default App;
