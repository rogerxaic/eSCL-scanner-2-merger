import React from "react";
import { ScanRequest } from "../types";

const DEFAULT_RECIPIENTS = ["scanner@example.com", "user@example.com"];

interface ScanOptionsProps {
    scanRequest: ScanRequest;
    onUpdate: (updatedRequest: ScanRequest) => void;
}

export function ScanOptions({
    scanRequest,
    onUpdate,
}: ScanOptionsProps): React.ReactElement {
    const handleAddRecipient = (input: HTMLInputElement) => {
        const value = input.value.trim();
        if (value && !scanRequest.recipients.includes(value)) {
            onUpdate({
                ...scanRequest,
                recipients: [...scanRequest.recipients, value],
            });
            input.value = "";
        }
    };

    return (
        <div>
            <div>
                <p>Recipients:</p>
                <label>
                    <input
                        type="checkbox"
                        checked={scanRequest.paperlessInstances.includes(
                            "papers-a.example.com"
                        )}
                        onChange={(e) =>
                            onUpdate({
                                ...scanRequest,
                                paperlessInstances: e.target.checked
                                    ? [
                                          ...scanRequest.paperlessInstances,
                                          "papers-a.example.com",
                                      ]
                                    : scanRequest.paperlessInstances.filter(
                                          (i) => i !== "papers-a.example.com"
                                      ),
                            })
                        }
                    />
                    Paperless (home)
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={scanRequest.paperlessInstances.includes(
                            "papers-b.example.com"
                        )}
                        onChange={(e) =>
                            onUpdate({
                                ...scanRequest,
                                paperlessInstances: e.target.checked
                                    ? [
                                          ...scanRequest.paperlessInstances,
                                          "papers-b.example.com",
                                      ]
                                    : scanRequest.paperlessInstances.filter(
                                          (i) => i !== "papers-b.example.com"
                                      ),
                            })
                        }
                    />
                    Paperless (site-b)
                </label>
                {DEFAULT_RECIPIENTS.map((recipient) => (
                    <label key={recipient}>
                        <style>
                            {`
                                label {
                                    display: flex;
                                    align-items: center;
                                    gap: 0.5rem;
                                    border: 2px solid #ccc;
                                    border-radius: 4px;
                                    padding: 0.5rem 1rem;
                                    margin: 0.5rem 0;
                                    cursor: pointer;
                                }
                                label:has(input:checked) {
                                    border: 4px solid #0066cc;
                                    padding: calc(0.5rem - 2px) calc(1rem - 2px);
                                }
                            `}
                        </style>
                        <input
                            type="checkbox"
                            checked={scanRequest.recipients.includes(recipient)}
                            onChange={(e) => {
                                const newRecipients = e.target.checked
                                    ? [...scanRequest.recipients, recipient]
                                    : scanRequest.recipients.filter(
                                          (r) => r !== recipient
                                      );
                                onUpdate({
                                    ...scanRequest,
                                    recipients: newRecipients,
                                });
                            }}
                        />
                        {recipient}
                    </label>
                ))}

                <div style={{ display: "flex", alignItems: "center" }}>
                    <style>
                        {`
                            input[type="text"] {
                                padding: 0.5rem;
                                border: 2px solid #ccc;
                                border-radius: 4px;
                                margin-right: 0.5rem;
                            }
                            button {
                                padding: 0.5rem 1rem;
                                background-color: #0066cc;
                                color: white;
                                border: none;
                                border-radius: 4px;
                                cursor: pointer;
                            }
                            button:hover {
                                background-color: #0052a3;
                            }
                            div {
                                margin: 0.5rem 0;
                            }
                        `}
                    </style>
                    <input
                        style={{ width: "100%" }}
                        type="text"
                        placeholder="Add custom recipient"
                        onKeyUp={(e) => {
                            if (e.key === "Enter") {
                                e.preventDefault();
                                handleAddRecipient(
                                    e.target as HTMLInputElement
                                );
                            }
                        }}
                    />
                    <button
                        onClick={(e) => {
                            e.preventDefault();
                            handleAddRecipient(
                                e.currentTarget
                                    .previousElementSibling as HTMLInputElement
                            );
                        }}
                    >
                        +
                    </button>
                </div>

                <div>
                    {scanRequest.recipients
                        .filter((r) => !DEFAULT_RECIPIENTS.includes(r))
                        .map((recipient) => (
                            <div key={recipient}>
                                {recipient}
                                <button
                                    onClick={() =>
                                        onUpdate({
                                            ...scanRequest,
                                            recipients:
                                                scanRequest.recipients.filter(
                                                    (r) => r !== recipient
                                                ),
                                        })
                                    }
                                >
                                    Remove
                                </button>
                            </div>
                        ))}
                </div>
            </div>
        </div>
    );
}
