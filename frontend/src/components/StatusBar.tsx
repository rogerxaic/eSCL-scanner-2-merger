import React from 'react';
import { ScannerStatus } from '../types'; // You'll need to adjust this import based on your types location

interface StatusBarProps {
    ws: WebSocket | null;
    scannerStatus: ScannerStatus | null;
}

export const StatusBar: React.FC<StatusBarProps> = ({ ws, scannerStatus }) => {
    return (
        <div
            className={`status-bar ${
                !ws
                    ? "status-bar--disconnected"
                    : scannerStatus?.scanning
                    ? "status-bar--scanning"
                    : "status-bar--connected"
            }`}
        >
            {ws
                ? scannerStatus?.scanning
                    ? "Scanning..."
                    : "Connected to scanner"
                : "Not connected to scanner"}
        </div>
    );
}; 