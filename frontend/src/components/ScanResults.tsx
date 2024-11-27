import React from "react";
import "./ScanResults.css";
import "./Modal.css";
import { ScanOptions } from "./ScanOptions";

interface ScanResult {
    session: string;
    error: string;
    thumbnail: string;
    pdf: string;
}

interface ScanResultsProps {
    results: { [key: string]: ScanResult };
    apiBaseUrl: string;
}

const ScanResults: React.FC<ScanResultsProps> = ({ results, apiBaseUrl }) => {
    const [selectedResults, setSelectedResults] = React.useState<string[]>([]);

    const [emails, setEmails] = React.useState<string[]>([]);
    const [paperlessInstances, setPaperlessInstances] = React.useState<
        string[]
    >([]);

    const [showModal, setShowModal] = React.useState<
        { fn: () => void; name: string } | (() => void) | boolean | undefined
    >(undefined);

    const toggleSelection = (session: string) => {
        setSelectedResults((prev) =>
            prev.includes(session)
                ? prev.filter((id) => id !== session)
                : [...prev, session]
        );
    };

    const mergePDFs = async () => {
        if (selectedResults.length < 2) return;
        const sessions = [...selectedResults];
        setSelectedResults([]);

        try {
            const response = await fetch(`${apiBaseUrl}/api/merge`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ sessions }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
        } catch (error) {
            console.error("Error merging PDFs:", error);
        }
    };

    const splitPDF = async () => {
        if (selectedResults.length !== 1) return;
        const session = selectedResults[0];
        setSelectedResults([]);

        try {
            const response = await fetch(`${apiBaseUrl}/api/split`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ session }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
        } catch (error) {
            console.error("Error splitting PDF:", error);
        }
    };

    const handleSendSelected = () => {
        if (selectedResults.length === 0) return;

        if (emails.length === 0 && paperlessInstances.length === 0) {
            setShowModal({ fn: sendSelected, name: "Send Selected PDFs" });
            return;
        }

        sendSelected();
    };

    const sendSelected = async () => {
        try {
            const response = await fetch(`${apiBaseUrl}/api/send`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    sessions: selectedResults,
                    recipients: emails,
                    paperlessInstances,
                }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            setSelectedResults([]);
        } catch (error) {
            console.error("Error sending documents:", error);
            alert("Error sending documents. Please try again.");
        }
    };

    return (
        <div className="scan-results">
            <div style={{ marginBottom: "1rem" }}>
                <button
                    onClick={mergePDFs}
                    disabled={selectedResults.length < 2}
                    className={`scan-button merge ${
                        selectedResults.length < 2 ? "disabled" : ""
                    }`}
                >
                    Merge Selected ({selectedResults.length})
                </button>

                <button
                    onClick={splitPDF}
                    disabled={selectedResults.length !== 1}
                    className={`scan-button split ${
                        selectedResults.length !== 1 ? "disabled" : ""
                    }`}
                >
                    Split PDF
                </button>

                <button
                    onClick={() => setSelectedResults([])}
                    disabled={selectedResults.length === 0}
                    className={`scan-button clear ${
                        selectedResults.length === 0 ? "disabled" : ""
                    }`}
                >
                    Clear Selection
                </button>
                <button
                    onClick={handleSendSelected}
                    disabled={selectedResults.length === 0}
                    className={`scan-button send ${
                        selectedResults.length === 0 ? "disabled" : ""
                    }`}
                >
                    Send Selected
                </button>
                <button
                    className="scan-button settings"
                    onClick={() => {
                        setShowModal(true);
                    }}
                >
                    ⚙️
                </button>
            </div>

            <div className={`modal ${showModal !== undefined ? "show" : ""}`}>
                <div className="modal-content">
                    <button
                        className="modal-close"
                        onClick={() => {
                            setShowModal(undefined);
                        }}
                    >
                        ×
                    </button>
                    <ScanOptions
                        onPaperlessInstancesChange={setPaperlessInstances}
                        onEmailsChange={setEmails}
                    />
                    {typeof showModal === "object" && (
                        <button
                            onClick={() => {
                                showModal.fn();
                                setShowModal(undefined);
                            }}
                        >
                            {showModal.name}
                        </button>
                    )}
                </div>
            </div>

            <div className="scan-results-grid">
                {Object.values(results).map((result) => (
                    <div
                        key={result.session}
                        className={`scan-result ${
                            selectedResults.includes(result.session)
                                ? "selected"
                                : ""
                        }`}
                        onClick={() => toggleSelection(result.session)}
                    >
                        <div className="session-id">
                            Session: {result.session}
                        </div>
                        {result.error ? (
                            <div className="error">{result.error}</div>
                        ) : (
                            <>
                                <div className="thumbnail-container">
                                    <img
                                        src={result.thumbnail}
                                        alt="Scan thumbnail"
                                        className="thumbnail"
                                    />
                                </div>
                                <div>
                                    <a
                                        href={result.pdf}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        onClick={(e) => e.stopPropagation()}
                                        className="download-link"
                                    >
                                        Download
                                    </a>
                                </div>
                            </>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
};

export default ScanResults;
