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
}

export interface ScannerStatus {
    scanning: boolean;
}

export interface BroadcastData {
    type: "scanResult" | "scanRequest" | "scannerStatus";
    scanResults?: ScanResult[];
    scanRequest?: ScanRequest;
    scannerStatus?: ScannerStatus;
}
