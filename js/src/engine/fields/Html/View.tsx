import React, { useMemo } from "react";
import CodeBlock from "@engine/fields/Markdown/controllers/CodeBlock";

interface HtmlViewProps {
  value?: string;
}

interface Part {
  html?: string;
  lang?: string;
  code?: string;
}

// goldmark renders a fenced block as <pre><code class="language-xxx">escaped text</code></pre>.
const FENCED = /<pre><code class="language-([\w+-]+)">([\s\S]*?)<\/code><\/pre>/g;

// DOMParser decodes entities without running anything.
const decode = (escaped: string) => new DOMParser().parseFromString(escaped, "text/html").documentElement.textContent || "";

// Splits compiled HTML into plain HTML runs and fenced code blocks, so the blocks can be highlighted by the shared CodeBlock (Prism, theme-aware).
function split(html: string): Part[] {
  const parts: Part[] = [];
  let last = 0;
  for (const m of html.matchAll(FENCED)) {
    if (m.index! > last) parts.push({ html: html.slice(last, m.index) });
    parts.push({ lang: m[1], code: decode(m[2]) });
    last = m.index! + m[0].length;
  }
  if (last < html.length) parts.push({ html: html.slice(last) });
  return parts;
}

// Renders already-compiled HTML (e.g. posts' compiled_html, produced
// server-side by goldmark's default escaping mode — no raw HTML from a
// markdown source ever passes through it).
const HtmlView: React.FC<HtmlViewProps> = ({ value }) => {
  const parts = useMemo(() => split(value || ""), [value]);
  if (!value) return null;
  return (
    <div className="field-html">
      {parts.map((p, i) =>
        p.code !== undefined ? (
          <CodeBlock key={i} className={`language-${p.lang}`}>
            {p.code}
          </CodeBlock>
        ) : (
          <div key={i} dangerouslySetInnerHTML={{ __html: p.html || "" }} />
        )
      )}
    </div>
  );
};

export default HtmlView;
