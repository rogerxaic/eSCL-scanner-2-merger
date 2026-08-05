import React, { useEffect, useState } from "react";
import "./ScanOptions.css";

const INSTANCE_LABELS: { [host: string]: string } = {
    "papers-a.example.com": "Paperless (home)",
    "papers-b.example.com": "Paperless (site-b)",
};

interface ScanOptionsProps {
    availableInstances: string[];
    defaultRecipients: string[];
    onPaperlessInstancesChange: (paperlessInstances: string[]) => void;
    onEmailsChange: (emails: string[]) => void;
}

export function ScanOptions({
    availableInstances,
    defaultRecipients,
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

    // Drop selections that are no longer configured in the settings.
    useEffect(() => {
        setPaperlessInstances((prev) =>
            prev.filter((i) => availableInstances.includes(i))
        );
    }, [availableInstances]);

    const handleAddRecipient = (input: HTMLInputElement) => {
        const value = input.value.trim();
        if (value && !emails.includes(value)) {
            setEmails([...emails, value]);
            input.value = "";
        }
    };

    return (
        <div className="scan-options">
            <p>Recipients:</p>
            {availableInstances.map((instance) => (
                <label key={instance}>
                    <input
                        type="checkbox"
                        checked={paperlessInstances.includes(instance)}
                        onChange={(e) =>
                            setPaperlessInstances(
                                e.target.checked
                                    ? [...paperlessInstances, instance]
                                    : paperlessInstances.filter(
                                          (i) => i !== instance
                                      )
                            )
                        }
                    />
                    {INSTANCE_LABELS[instance] ?? instance}
                </label>
            ))}
            {defaultRecipients.map((recipient) => (
                <label key={recipient}>
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

            <div className="scan-options-row">
                <input
                    style={{ width: "100%" }}
                    type="text"
                    placeholder="Add custom recipient"
                    onKeyUp={(e) => {
                        if (e.key === "Enter") {
                            e.preventDefault();
                            handleAddRecipient(e.target as HTMLInputElement);
                        }
                    }}
                />
                <button
                    type="button"
                    onClick={(e) => {
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
                    .filter((r) => !defaultRecipients.includes(r))
                    .map((recipient) => (
                        <div className="custom-recipient" key={recipient}>
                            {recipient}
                            <button
                                type="button"
                                onClick={() =>
                                    setEmails(
                                        emails.filter((r) => r !== recipient)
                                    )
                                }
                            >
                                Remove
                            </button>
                        </div>
                    ))}
            </div>
        </div>
    );
}
