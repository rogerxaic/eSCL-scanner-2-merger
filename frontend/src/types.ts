export type InputSource = "Platen" | "Feeder" | "Auto";

export interface ScanRequest {
    inputSource: InputSource;
    session: string;
    recipients: string[];
    paperlessInstances: string[];
    odooCompanies?: number[];
}

export interface ScanResult {
    session: string;
    error: string;
    thumbnail: string;
    pdf: string;
    createdAt: string;
    pages: number;
}

export interface ScannerStatus {
    scanning: boolean;
}

export interface PaperlessInstance {
    host: string;
    username: string;
    password: string;
    token: string;
    // The API never returns stored secrets, only whether one is set. Sending
    // back an empty password/token keeps whatever is already stored.
    hasPassword?: boolean;
    hasToken?: boolean;
}

export interface OdooCompany {
    id: number;
    name: string;
}

export interface OdooConfig {
    url: string;
    database: string;
    // Like the paperless secrets: never returned, blank keeps the stored key.
    apiKey: string;
    hasApiKey?: boolean;
    confidentiality: string;
    companies: OdooCompany[];
}

export interface Settings {
    retentionDays: number;
    emailRecipients: string[];
    paperlessInstances: PaperlessInstance[];
    odoo: OdooConfig;
}

export interface BroadcastData {
    type: "scanResult" | "scanRequest" | "scannerStatus";
    scanResults?: ScanResult[];
    scanRequest?: ScanRequest;
    scannerStatus?: ScannerStatus;
}
