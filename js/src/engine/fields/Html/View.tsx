import React from "react";

interface HtmlViewProps {
  value?: string;
}

// Renders already-compiled HTML (e.g. posts' compiled_html, produced
// server-side by goldmark's default escaping mode — no raw HTML from a
// markdown source ever passes through it).
const HtmlView: React.FC<HtmlViewProps> = ({ value }) => {
  if (!value) return null;
  return <div className="field-html" dangerouslySetInnerHTML={{ __html: value }} />;
};

export default HtmlView;
