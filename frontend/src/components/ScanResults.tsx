import React from "react";

interface ScanResult {
    session: string;
    error: string;
    thumbnail: string;
    pdf: string;
}

interface ScanResultsProps {
    results: { [key: string]: ScanResult };
}

const ScanResults: React.FC<ScanResultsProps> = ({ results }) => {
    const [selectedResults, setSelectedResults] = React.useState<string[]>([]);

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
            const response = await fetch("http://localhost:8080/api/merge", {
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
            const response = await fetch("http://localhost:8080/api/split", {
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

    return (
        <div className="scan-results">
            <div style={{ marginBottom: "1rem" }}>
                <button
                    onClick={mergePDFs}
                    disabled={selectedResults.length < 2}
                    style={{
                        marginRight: "1rem",
                        padding: "0.5rem 1rem",
                        backgroundColor:
                            selectedResults.length < 2 ? "#ccc" : "#0066cc",
                        color: "white",
                        border: "none",
                        borderRadius: "4px",
                        cursor:
                            selectedResults.length < 2
                                ? "not-allowed"
                                : "pointer",
                    }}
                >
                    Merge Selected ({selectedResults.length})
                </button>

                <button
                    onClick={splitPDF}
                    disabled={selectedResults.length !== 1}
                    style={{
                        marginRight: "1rem",
                        padding: "0.5rem 1rem",
                        backgroundColor:
                            selectedResults.length !== 1 ? "#ccc" : "#0066cc",
                        color: "white",
                        border: "none",
                        borderRadius: "4px",
                        cursor:
                            selectedResults.length !== 1
                                ? "not-allowed"
                                : "pointer",
                    }}
                >
                    Split PDF
                </button>

                <button
                    onClick={() => setSelectedResults([])}
                    disabled={selectedResults.length === 0}
                    style={{
                        padding: "0.5rem 1rem",
                        backgroundColor:
                            selectedResults.length === 0 ? "#ccc" : "#dc3545",
                        color: "white",
                        border: "none",
                        borderRadius: "4px",
                        cursor:
                            selectedResults.length === 0
                                ? "not-allowed"
                                : "pointer",
                    }}
                >
                    Clear Selection
                </button>
            </div>
            
                <div className="scan-results-grid" style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(4, 1fr)',
                    gap: '16px',
                    width: '100%'
                }}>
                    {Object.values(results).map((result) => (
                        <div
                            key={result.session}
                            className={`scan-result ${
                                selectedResults.includes(result.session)
                                    ? "selected"
                                    : ""
                            }`}
                            onClick={() => toggleSelection(result.session)}
                            style={{
                                boxSizing: "border-box",
                                border: selectedResults.includes(result.session)
                                    ? "4px solid #0066cc"
                                    : "2px solid #666",
                                borderRadius: "8px",
                                padding: selectedResults.includes(result.session)
                                    ? "14px"
                                    : "16px",
                                backgroundColor: selectedResults.includes(
                                    result.session
                                )
                                    ? "#e0e0e0"
                                    : "#f0f0f0",
                                color: "#333",
                                cursor: "pointer",
                                display: "flex",
                                flexDirection: "column",
                                alignItems: "center",
                                justifyContent: "space-between",
                                aspectRatio: "1 / 1",
                                width: "calc(100% - 16px)",
                            }}
                        >
                            <div className="session-id" style={{ fontSize: "0.6em" }}>
                                Session: {result.session}
                            </div>
                            {result.error ? (
                                <div className="error" style={{ fontSize: "0.8em" }}>
                                    {result.error}
                                </div>
                            ) : (
                                <>
                                    <div
                                        style={{
                                            border: "1px solid #ccc",
                                            padding: "8px",
                                            backgroundColor: "#fff",
                                            display: "flex",
                                            justifyContent: "center",
                                            alignItems: "center",
                                            width: "80%",
                                            height: "80%",
                                        }}
                                    >
                                        <img
                                            src={result.thumbnail}
                                            alt="Scan thumbnail"
                                            style={{
                                                maxWidth: "100%",
                                                maxHeight: "100%",
                                                objectFit: "contain",
                                            }}
                                        />
                                    </div>
                                    <div>
                                        <a
                                            href={result.pdf}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            onClick={(e) => e.stopPropagation()}
                                            style={{
                                                color: "#0066cc",
                                                textDecoration: "none",
                                                fontWeight: "bold",
                                                fontSize: "0.8em",
                                            }}
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
