import React from "react";

interface HtmlEditProps {
  value?: string;
  disabled?: boolean;
  onChange?: (value: string) => void;
}

const HtmlEdit: React.FC<HtmlEditProps> = ({ value, disabled, onChange }) => (
  <textarea
    className="form-control"
    style={{ fontFamily: "monospace", minHeight: 200 }}
    value={value ?? ""}
    disabled={disabled}
    onChange={(e) => onChange?.(e.target.value)}
  />
);

export default HtmlEdit;
