import React, { useState } from 'react';
import { InputSource } from '../types';

const SOURCES: { value: InputSource; label: string }[] = [
    { value: "Auto", label: "Auto" },
    { value: "Platen", label: "Glass" },
    { value: "Feeder", label: "Top Tray" },
];

interface InputSourceSelectorProps {
    onScan: (inputSource: InputSource) => void;
    disabled: boolean;
}

export function InputSourceSelector({ onScan, disabled }: InputSourceSelectorProps): React.ReactElement {
    const [selectedSource, setSelectedSource] = useState<InputSource>("Auto");

    return (
        <div className="scan-controls">
            <p className="scan-controls__label" id="input-source-label">
                Input Source:
            </p>
            <div className="scan-controls__row">
                <div
                    className="segmented"
                    role="radiogroup"
                    aria-labelledby="input-source-label"
                >
                    {SOURCES.map(({ value, label }) => (
                        <label
                            key={value}
                            className={`segmented__option ${
                                selectedSource === value ? "is-active" : ""
                            }`}
                        >
                            <input
                                type="radio"
                                name="inputSource"
                                value={value}
                                checked={selectedSource === value}
                                onChange={() => setSelectedSource(value)}
                            />
                            {label}
                        </label>
                    ))}
                </div>
                <button
                    type="button"
                    className="btn btn--primary"
                    onClick={() => onScan(selectedSource)}
                    disabled={disabled}
                >
                    Scan
                </button>
            </div>
        </div>
    );
}
