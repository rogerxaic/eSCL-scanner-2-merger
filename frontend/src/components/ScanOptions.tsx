import React, { useEffect, useState } from "react";

const DEFAULT_RECIPIENTS = ["scanner@example.com", "user@example.com"];

interface ScanOptionsProps {
    onPaperlessInstancesChange: (paperlessInstances: string[]) => void;
    onEmailsChange: (emails: string[]) => void;
}

export function ScanOptions({
    onPaperlessInstancesChange,
    onEmailsChange,
}: ScanOptionsProps): React.ReactElement {
    const [emails, setEmails] = useState<string[]>([]);
    const [paperlessInstances, setPaperlessInstances] = useState<string[]>([]);

    useEffect(() => {
        onEmailsChange(emails);
        onPaperlessInstancesChange(paperlessInstances);
    }, [
        emails,
        paperlessInstances,
        onEmailsChange,
        onPaperlessInstancesChange,
    ]);

    const handleAddRecipient = (input: HTMLInputElement) => {
        const value = input.value.trim();
        if (value && !emails.includes(value)) {
            setEmails([...emails, value]);
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
                        checked={paperlessInstances.includes("papers-a.example.com")}
                        onChange={(e) =>
                            setPaperlessInstances(
                                e.target.checked
                                    ? [...paperlessInstances, "papers-a.example.com"]
                                    : paperlessInstances.filter(
                                          (i) => i !== "papers-a.example.com"
                                      )
                            )
                        }
                    />
                    Paperless (home)
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={paperlessInstances.includes(
                            "papers-b.example.com"
                        )}
                        onChange={(e) =>
                            setPaperlessInstances(
                                e.target.checked
                                    ? [
                                          ...paperlessInstances,
                                          "papers-b.example.com",
                                      ]
                                    : paperlessInstances.filter(
                                          (i) => i !== "papers-b.example.com"
                                      )
                            )
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
                            checked={emails.includes(recipient)}
                            onChange={(e) => {
                                const newRecipients = e.target.checked
                                    ? [...emails, recipient]
                                    : emails.filter((r) => r !== recipient);
                                setEmails(newRecipients);
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
                    {emails
                        .filter((r) => !DEFAULT_RECIPIENTS.includes(r))
                        .map((recipient) => (
                            <div key={recipient}>
                                {recipient}
                                <button
                                    onClick={() =>
                                        setEmails(
                                            emails.filter(
                                                (r) => r !== recipient
                                            )
                                        )
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
