import React from "react";

interface HtmlListProps {
  value?: string;
}

const MAX_LEN = 140;

function stripHtml(html: string): string {
  const text = html.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();
  return text.length > MAX_LEN ? text.slice(0, MAX_LEN) + "…" : text;
}

const HtmlList: React.FC<HtmlListProps> = ({ value }) => <span>{value ? stripHtml(value) : ""}</span>;

export default HtmlList;
