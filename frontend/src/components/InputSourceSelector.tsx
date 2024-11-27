import React, { useState } from 'react';
import { InputSource } from '../types';

interface InputSourceSelectorProps {
    onScan: (inputSource: InputSource) => void;
    disabled: boolean;
}

export function InputSourceSelector({ onScan, disabled }: InputSourceSelectorProps): React.ReactElement {
    const [selectedSource, setSelectedSource] = useState<InputSource>("Auto");

    return (
        <div className="input-source-container">
            <p>Input Source:</p>
            <div className="input-source-options">
                {["Auto", "Platen", "Feeder"].map((source) => (
                    <div
                        key={source}
                        className={`input-source-tile ${
                            selectedSource === source ? "selected" : ""
                        }`}
                        onClick={() => setSelectedSource(source as "Auto" | "Platen" | "Feeder")}
                    >
                        <input
                            type="radio"
                            name="inputSource"
                            value={source}
                            checked={selectedSource === source}
                            onChange={() => {}} // Handled by onClick
                        />
                        <span>
                            {source === "Auto" ? "Auto" :
                             source === "Platen" ? "Glass" :
                             source === "Feeder" ? "Top tray" : source}
                        </span>
                    </div>
                ))}
                <button onClick={() => onScan(selectedSource)} disabled={disabled}>Scan</button>
            </div>
            <style>{`
                .input-source-container {
                    margin: 1rem 0;
                }
                .input-source-options {
                    display: flex;
                    gap: 1rem;
                    justify-content: center;
                }
                .input-source-tile {
                    border: 2px solid #ccc;
                    border-radius: 4px;
                    padding: 0.5rem 1rem;
                    cursor: pointer;
                    display: flex;
                    align-items: center;
                    gap: 0.5rem;
                }
                .input-source-tile.selected {
                    border-width: 3px;
                    border-color: #007bff;
                }
                .input-source-tile input[type="radio"] {
                    margin: 0;
                }
            `}</style>
        </div>
    );
} 