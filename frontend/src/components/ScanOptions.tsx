import React, { useEffect, useState } from "react";
import { OdooCompany } from "../types";
import "./ScanOptions.css";

interface ScanOptionsProps {
    availableInstances: string[];
    availableOdooCompanies: OdooCompany[];
    defaultRecipients: string[];
    onPaperlessInstancesChange: (paperlessInstances: string[]) => void;
    onOdooCompaniesChange: (companies: number[]) => void;
    onEmailsChange: (emails: string[]) => void;
}

export function ScanOptions({
    availableInstances,
    availableOdooCompanies,
    defaultRecipients,
    onPaperlessInstancesChange,
    onOdooCompaniesChange,
    onEmailsChange,
}: ScanOptionsProps): React.ReactElement {
    const [emails, setEmails] = useState<string[]>([]);
    const [paperlessInstances, setPaperlessInstances] = useState<string[]>([]);
    const [odooCompanies, setOdooCompanies] = useState<number[]>([]);

    useEffect(() => {
        onOdooCompaniesChange(odooCompanies);
    }, [odooCompanies, onOdooCompaniesChange]);

    useEffect(() => {
        // Keep the same array when nothing is dropped, so the parent is not
        // updated on every render.
        setOdooCompanies((prev) => {
            const kept = prev.filter((id) =>
                availableOdooCompanies.some((c) => c.id === id)
            );
            return kept.length === prev.length ? prev : kept;
        });
    }, [availableOdooCompanies]);

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
                    {instance}
                </label>
            ))}
            {availableOdooCompanies.map((company) => (
                <label key={`odoo-${company.id}`}>
                    <input
                        type="checkbox"
                        checked={odooCompanies.includes(company.id)}
                        onChange={(e) =>
                            setOdooCompanies(
                                e.target.checked
                                    ? [...odooCompanies, company.id]
                                    : odooCompanies.filter(
                                          (id) => id !== company.id
                                      )
                            )
                        }
                    />
                    Odoo · {company.name}
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
