export type InputSource = "Platen" | "Feeder" | "Auto";

export interface ScanRequest {
    inputSource: InputSource;
    session: string;
    recipients: string[];
    paperlessInstances: string[];
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

export interface Settings {
    retentionDays: number;
    emailRecipients: string[];
    paperlessInstances: PaperlessInstance[];
}

export interface BroadcastData {
    type: "scanResult" | "scanRequest" | "scannerStatus";
    scanResults?: ScanResult[];
    scanRequest?: ScanRequest;
    scannerStatus?: ScannerStatus;
}
