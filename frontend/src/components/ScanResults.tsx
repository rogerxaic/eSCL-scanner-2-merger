import React from "react";
import "./ScanResults.css";
import "./Modal.css";
import { ScanOptions } from "./ScanOptions";
import { SettingsPanel } from "./SettingsPanel";
import { ScanResult, Settings } from "../types";

interface ScanResultsProps {
    results: { [key: string]: ScanResult };
    apiBaseUrl: string;
}

function formatCreatedAt(createdAt: string): string {
    if (!createdAt) return "";
    const date = new Date(createdAt);
    if (isNaN(date.getTime())) return "";
    return date.toLocaleString([], {
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
    });
}

const ScanResults: React.FC<ScanResultsProps> = ({ results, apiBaseUrl }) => {
    const [selectedResults, setSelectedResults] = React.useState<string[]>([]);

    const [emails, setEmails] = React.useState<string[]>([]);
    const [paperlessInstances, setPaperlessInstances] = React.useState<
        string[]
    >([]);
    const [odooCompanies, setOdooCompanies] = React.useState<number[]>([]);

    const [settings, setSettings] = React.useState<Settings | null>(null);
    const [errorMessage, setErrorMessage] = React.useState<string | null>(
        null
    );
    const [sending, setSending] = React.useState<boolean>(false);

    const [showModal, setShowModal] = React.useState<
        { name: string } | boolean | undefined
    >(undefined);

    React.useEffect(() => {
        fetch(`${apiBaseUrl}/api/settings`)
            .then((r) => r.json())
            .then((s: Settings) => setSettings(s))
            .catch((error) =>
                console.error("Error loading settings:", error)
            );
    }, [apiBaseUrl]);

    const reportError = async (prefix: string, response?: Response) => {
        let detail = "";
        if (response) {
            try {
                const body = await response.json();
                detail = body.error ? `: ${body.error}` : "";
            } catch {
                detail = `: HTTP ${response.status}`;
            }
        }
        setErrorMessage(`${prefix}${detail}`);
    };

    const toggleSelection = (session: string) => {
        setSelectedResults((prev) =>
            prev.includes(session)
                ? prev.filter((id) => id !== session)
                : [...prev, session]
        );
    };

    const mergePDFs = async () => {
        if (selectedResults.length !== 2) return;
        const sessions = [...selectedResults];
        setSelectedResults([]);
        setErrorMessage(null);

        try {
            const response = await fetch(`${apiBaseUrl}/api/merge`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ sessions }),
            });

            if (!response.ok) {
                await reportError("Combining failed", response);
            }
        } catch (error) {
            console.error("Error merging PDFs:", error);
            setErrorMessage("Combining failed: network error");
        }
    };

    const splitPDF = async () => {
        if (selectedResults.length !== 1) return;
        const session = selectedResults[0];
        setSelectedResults([]);
        setErrorMessage(null);

        try {
            const response = await fetch(`${apiBaseUrl}/api/split`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ session }),
            });

            if (!response.ok) {
                await reportError("Split failed", response);
            }
        } catch (error) {
            console.error("Error splitting PDF:", error);
            setErrorMessage("Split failed: network error");
        }
    };

    const deleteResult = async (session: string) => {
        if (!window.confirm("Delete this scan?")) return;
        setErrorMessage(null);
        try {
            const response = await fetch(`${apiBaseUrl}/api/delete`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ sessions: [session] }),
            });
            if (!response.ok) {
                await reportError("Delete failed", response);
                return;
            }
            setSelectedResults((prev) => prev.filter((s) => s !== session));
        } catch (error) {
            console.error("Error deleting scan:", error);
            setErrorMessage("Delete failed: network error");
        }
    };

    const handleSendSelected = () => {
        if (selectedResults.length === 0) return;

        // Always confirm the destination. Reusing the previous choice silently
        // sent documents somewhere the user had not picked for *this* batch,
        // and a successful send drops them from the list — so the mistake was
        // invisible until the scans were gone.
        setShowModal({
            name: "Send Selected PDFs",
        });
    };

    const sendSelected = async () => {
        if (sending) return;
        setSending(true);
        setErrorMessage(null);
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
                    odooCompanies,
                }),
            });

            if (!response.ok) {
                await reportError("Sending failed", response);
                return;
            }

            setSelectedResults([]);
        } catch (error) {
            console.error("Error sending documents:", error);
            setErrorMessage("Sending failed: network error");
        } finally {
            setSending(false);
        }
    };

    const sortedResults = Object.values(results).sort((a, b) =>
        (a.createdAt || "").localeCompare(b.createdAt || "")
    );

    return (
        <div className="scan-results">
            <div className="action-bar">
                <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={splitPDF}
                    disabled={selectedResults.length !== 1}
                >
                    Split Selected Pages
                </button>
                <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={mergePDFs}
                    disabled={selectedResults.length !== 2}
                    title="Interleaves the two selected PDFs: pages of the first, then the second in reverse (recto/verso)"
                >
                    Combine Selected Duplex Pages ({selectedResults.length}/2)
                </button>
                <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={() => setSelectedResults([])}
                    disabled={selectedResults.length === 0}
                >
                    Clear Selection
                </button>
                <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={handleSendSelected}
                    disabled={selectedResults.length === 0}
                >
                    Send Selected
                </button>
                <button
                    type="button"
                    className="btn btn--secondary btn--icon"
                    aria-label="Settings"
                    onClick={() => {
                        setShowModal(true);
                    }}
                >
                    ⚙️
                </button>
            </div>

            {errorMessage && (
                <div className="error-banner">
                    {errorMessage}
                    <button
                        type="button"
                        onClick={() => setErrorMessage(null)}
                    >
                        ×
                    </button>
                </div>
            )}

            <div className={`modal ${showModal !== undefined ? "show" : ""}`}>
                <div className="modal-content">
                    <button
                        type="button"
                        className="modal-close"
                        onClick={() => {
                            setShowModal(undefined);
                        }}
                    >
                        ×
                    </button>
                    {showModal === true ? (
                        <SettingsPanel
                            apiBaseUrl={apiBaseUrl}
                            settings={settings}
                            onSaved={setSettings}
                        />
                    ) : (
                        <>
                            <ScanOptions
                                availableInstances={(
                                    settings?.paperlessInstances ?? []
                                ).map((i) => i.host)}
                                availableOdooCompanies={
                                    settings?.odoo?.url
                                        ? settings.odoo.companies ?? []
                                        : []
                                }
                                onOdooCompaniesChange={setOdooCompanies}
                                defaultRecipients={
                                    settings?.emailRecipients ?? []
                                }
                                onPaperlessInstancesChange={
                                    setPaperlessInstances
                                }
                                onEmailsChange={setEmails}
                            />
                            {typeof showModal === "object" &&
                                showModal !== null && (
                                    <button
                                        type="button"
                                        className="btn btn--primary modal-send"
                                        disabled={sending}
                                        onClick={() => {
                                            sendSelected();
                                            setShowModal(undefined);
                                        }}
                                    >
                                        {sending ? "Sending…" : showModal.name}
                                    </button>
                                )}
                        </>
                    )}
                </div>
            </div>

            {sortedResults.length === 0 && (
                <p className="empty-state">
                    No scans yet — place a document on the scanner and press
                    Scan.
                </p>
            )}

            <div className="scan-results-grid">
                {sortedResults.map((result) => {
                    const selectionIndex = selectedResults.indexOf(
                        result.session
                    );
                    return (
                        <div
                            key={result.session}
                            className={`scan-result ${
                                selectionIndex >= 0 ? "selected" : ""
                            }`}
                            onClick={() => toggleSelection(result.session)}
                            title={`Session ${result.session}`}
                        >
                            {selectionIndex >= 0 && (
                                <span className="selection-badge">
                                    {selectionIndex + 1}
                                </span>
                            )}
                            <div className="card-header">
                                <input
                                    type="checkbox"
                                    className="card-checkbox"
                                    checked={selectionIndex >= 0}
                                    aria-label="Select scan"
                                    onClick={(e) => e.stopPropagation()}
                                    onChange={() =>
                                        toggleSelection(result.session)
                                    }
                                />
                                <span className="scan-meta">
                                    {formatCreatedAt(result.createdAt) ||
                                        result.session.slice(0, 8)}
                                    {result.pages > 0 &&
                                        ` · ${result.pages} page${
                                            result.pages > 1 ? "s" : ""
                                        }`}
                                </span>
                                <button
                                    type="button"
                                    className="delete-button"
                                    title="Delete this scan"
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        deleteResult(result.session);
                                    }}
                                >
                                    🗑
                                </button>
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
                                    <div className="scan-links">
                                        <a
                                            href={`${result.pdf}?inline=1`}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            onClick={(e) =>
                                                e.stopPropagation()
                                            }
                                            className="download-link"
                                        >
                                            View
                                        </a>
                                        <a
                                            href={result.pdf}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            onClick={(e) =>
                                                e.stopPropagation()
                                            }
                                            className="download-link"
                                        >
                                            Download
                                        </a>
                                    </div>
                                </>
                            )}
                        </div>
                    );
                })}
            </div>
        </div>
    );
};

export default ScanResults;
