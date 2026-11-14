import React, { useEffect, useState } from "react";
import { OdooConfig, PaperlessInstance, Settings } from "../types";

const emptyOdoo: OdooConfig = {
    url: "",
    database: "",
    apiKey: "",
    confidentiality: "internal",
    companies: [],
};
import "./SettingsPanel.css";

interface SettingsPanelProps {
    apiBaseUrl: string;
    settings: Settings | null;
    onSaved: (settings: Settings) => void;
}

export function SettingsPanel({
    apiBaseUrl,
    settings,
    onSaved,
}: SettingsPanelProps): React.ReactElement {
    const [days, setDays] = useState<string>("");
    const [emails, setEmails] = useState<string[]>([]);
    const [newEmail, setNewEmail] = useState<string>("");
    const [instances, setInstances] = useState<PaperlessInstance[]>([]);
    const [newInstance, setNewInstance] = useState<PaperlessInstance>({
        host: "",
        username: "",
        password: "",
        token: "",
    });
    const [odoo, setOdoo] = useState<OdooConfig>(emptyOdoo);
    const [odooLoad, setOdooLoad] = useState<string | null>(null);
    const [status, setStatus] = useState<
        "idle" | "saving" | "saved" | "error"
    >("idle");

    useEffect(() => {
        if (settings) {
            setDays(String(settings.retentionDays));
            setEmails(settings.emailRecipients);
            setInstances(settings.paperlessInstances);
            setOdoo({ ...emptyOdoo, ...(settings.odoo ?? {}), apiKey: "" });
        }
    }, [settings]);

    const addEmail = () => {
        const value = newEmail.trim();
        if (value && !emails.includes(value)) {
            setEmails([...emails, value]);
            setNewEmail("");
            setStatus("idle");
        }
    };

    const addInstance = () => {
        const host = newInstance.host
            .trim()
            .replace(/^https?:\/\//, "")
            .replace(/\/$/, "");
        if (!host || instances.some((i) => i.host === host)) return;
        setInstances([...instances, { ...newInstance, host }]);
        setNewInstance({ host: "", username: "", password: "", token: "" });
        setStatus("idle");
    };

    const updateInstance = (
        index: number,
        field: keyof PaperlessInstance,
        value: string
    ) => {
        setInstances(
            instances.map((instance, i) =>
                i === index ? { ...instance, [field]: value } : instance
            )
        );
        setStatus("idle");
    };

    const updateOdoo = (changes: Partial<OdooConfig>) => {
        setOdoo({ ...odoo, ...changes });
        setStatus("idle");
    };

    // Reads the companies with the key that is saved on the server, so save
    // the URL and key first.
    const loadOdooCompanies = async () => {
        setOdooLoad("Loading…");
        try {
            const response = await fetch(`${apiBaseUrl}/api/odoo/companies`);
            const body = await response.json();
            if (!response.ok) {
                setOdooLoad(body.error ?? `HTTP ${response.status}`);
                return;
            }
            updateOdoo({ companies: body });
            setOdooLoad(
                `${body.length} companies loaded. Remove the ones you do not want, then save.`
            );
        } catch (error) {
            setOdooLoad("Network error");
        }
    };

    const save = async () => {
        const parsed = parseInt(days, 10);
        if (isNaN(parsed) || parsed < 1 || parsed > 365) {
            setStatus("error");
            return;
        }
        setStatus("saving");
        try {
            const response = await fetch(`${apiBaseUrl}/api/settings`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    retentionDays: parsed,
                    emailRecipients: emails,
                    paperlessInstances: instances,
                    odoo,
                }),
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const saved = (await response.json()) as Settings;
            onSaved(saved);
            setOdoo({ ...emptyOdoo, ...(saved.odoo ?? {}), apiKey: "" });
            setStatus("saved");
        } catch (error) {
            console.error("Error saving settings:", error);
            setStatus("error");
        }
    };

    return (
        <div className="settings-panel">
            <h3>Settings</h3>

            <div className="settings-section">
                <p>Delete scans after:</p>
                <div className="settings-row">
                    <input
                        type="number"
                        min={1}
                        max={365}
                        value={days}
                        disabled={settings === null}
                        onChange={(e) => {
                            setDays(e.target.value);
                            setStatus("idle");
                        }}
                    />
                    <span>day(s)</span>
                </div>
            </div>

            <div className="settings-section">
                <p>Email recipients (shown in the send dialog):</p>
                {emails.length === 0 && (
                    <p className="settings-hint">No recipients configured.</p>
                )}
                {emails.map((email) => (
                    <div className="settings-instance" key={email}>
                        <span>{email}</span>
                        <button
                            type="button"
                            onClick={() => {
                                setEmails(emails.filter((e) => e !== email));
                                setStatus("idle");
                            }}
                        >
                            Remove
                        </button>
                    </div>
                ))}
                <div className="settings-row">
                    <input
                        type="email"
                        placeholder="someone@example.com"
                        value={newEmail}
                        onChange={(e) => setNewEmail(e.target.value)}
                        onKeyUp={(e) => {
                            if (e.key === "Enter") {
                                e.preventDefault();
                                addEmail();
                            }
                        }}
                    />
                    <button type="button" onClick={addEmail}>
                        +
                    </button>
                </div>
            </div>

            <div className="settings-section">
                <p>Paperless instances:</p>
                {instances.length === 0 && (
                    <p className="settings-hint">No instances configured.</p>
                )}
                {instances.map((instance, index) => (
                    <div className="settings-instance-form" key={instance.host}>
                        <div className="settings-instance-header">
                            <strong>{instance.host}</strong>
                            <button
                                type="button"
                                onClick={() => {
                                    setInstances(
                                        instances.filter(
                                            (i) => i.host !== instance.host
                                        )
                                    );
                                    setStatus("idle");
                                }}
                            >
                                Remove
                            </button>
                        </div>
                        <div className="settings-row">
                            <input
                                type="text"
                                placeholder="Username"
                                autoComplete="off"
                                value={instance.username}
                                onChange={(e) =>
                                    updateInstance(
                                        index,
                                        "username",
                                        e.target.value
                                    )
                                }
                            />
                            <input
                                type="password"
                                placeholder={
                                    instance.hasPassword
                                        ? "Password (set — blank keeps it)"
                                        : "Password"
                                }
                                autoComplete="new-password"
                                value={instance.password ?? ""}
                                onChange={(e) =>
                                    updateInstance(
                                        index,
                                        "password",
                                        e.target.value
                                    )
                                }
                            />
                        </div>
                        <div className="settings-row">
                            <input
                                type="password"
                                placeholder={
                                    instance.hasToken
                                        ? "API token (set — blank keeps it)"
                                        : "API token (needed if MFA is on)"
                                }
                                autoComplete="new-password"
                                value={instance.token ?? ""}
                                onChange={(e) =>
                                    updateInstance(
                                        index,
                                        "token",
                                        e.target.value
                                    )
                                }
                            />
                        </div>
                    </div>
                ))}
                <div className="settings-instance-form">
                    <div className="settings-row">
                        <input
                            type="text"
                            placeholder="papers.example.com"
                            value={newInstance.host}
                            onChange={(e) =>
                                setNewInstance({
                                    ...newInstance,
                                    host: e.target.value,
                                })
                            }
                        />
                    </div>
                    <div className="settings-row">
                        <input
                            type="text"
                            placeholder="Username"
                            autoComplete="off"
                            value={newInstance.username}
                            onChange={(e) =>
                                setNewInstance({
                                    ...newInstance,
                                    username: e.target.value,
                                })
                            }
                        />
                        <input
                            type="password"
                            placeholder="Password"
                            autoComplete="new-password"
                            value={newInstance.password}
                            onChange={(e) =>
                                setNewInstance({
                                    ...newInstance,
                                    password: e.target.value,
                                })
                            }
                        />
                        <input
                            type="password"
                            placeholder="API token (optional)"
                            autoComplete="new-password"
                            value={newInstance.token}
                            onChange={(e) =>
                                setNewInstance({
                                    ...newInstance,
                                    token: e.target.value,
                                })
                            }
                        />
                        <button type="button" onClick={addInstance}>
                            +
                        </button>
                    </div>
                </div>
            </div>

            <div className="settings-section">
                <p>Odoo (archives scans for one company):</p>
                <div className="settings-row">
                    <input
                        type="text"
                        placeholder="odoo.example.com"
                        value={odoo.url}
                        onChange={(e) => updateOdoo({ url: e.target.value })}
                    />
                    <input
                        type="text"
                        placeholder="Database (optional)"
                        value={odoo.database}
                        onChange={(e) =>
                            updateOdoo({ database: e.target.value })
                        }
                    />
                </div>
                <div className="settings-row">
                    <input
                        type="password"
                        placeholder={
                            odoo.hasApiKey
                                ? "API key (set — blank keeps it)"
                                : "API key of the scanner agent"
                        }
                        autoComplete="new-password"
                        value={odoo.apiKey}
                        onChange={(e) => updateOdoo({ apiKey: e.target.value })}
                    />
                    <select
                        value={odoo.confidentiality || "internal"}
                        onChange={(e) =>
                            updateOdoo({ confidentiality: e.target.value })
                        }
                    >
                        <option value="internal">Internal</option>
                        <option value="accounting">Accounting</option>
                        <option value="private">Private</option>
                    </select>
                </div>
                {odoo.companies.map((company) => (
                    <div className="settings-instance" key={company.id}>
                        <span>
                            {company.name} (id {company.id})
                        </span>
                        <button
                            type="button"
                            onClick={() =>
                                updateOdoo({
                                    companies: odoo.companies.filter(
                                        (c) => c.id !== company.id
                                    ),
                                })
                            }
                        >
                            Remove
                        </button>
                    </div>
                ))}
                <div className="settings-row">
                    <button
                        type="button"
                        onClick={loadOdooCompanies}
                        disabled={!settings?.odoo?.url || !settings?.odoo?.hasApiKey}
                    >
                        Load companies from Odoo
                    </button>
                </div>
                {odooLoad && <p className="settings-hint">{odooLoad}</p>}
            </div>

            <div className="settings-row">
                <button
                    type="button"
                    onClick={save}
                    disabled={status === "saving" || settings === null}
                >
                    Save settings
                </button>
                {status === "saved" && <span>Saved ✓</span>}
                {status === "error" && <span>Failed to save</span>}
            </div>
        </div>
    );
}
