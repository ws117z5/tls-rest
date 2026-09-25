import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import MarkdownRender from "@engine/fields/Markdown/controllers/MarkdownRender";
import { t, subscribe } from "@engine/controllers/i18n";

interface MarkdownToolState {
  source: string;
  sourceWidthPct: number; // source column width, as % of the row (preview takes the rest)
}

const MIN_PCT = 10;
const MAX_PCT = 90; // symmetric with MIN_PCT, so the preview never drops below 10% either

// Same rendering pipeline posts/comments use (react-markdown + remark-gfm,
// KaTeX for $inline$/$$block$$ math, and the :::graph Graphviz directive) —
// see js/src/engine/fields/Markdown/controllers/MarkdownRender.tsx.
const SAMPLE = `# Markdown playground

Some **bold**, *italic*, and \`inline code\`.

\`\`\`go
func main() {
	fmt.Println("hello")
}
\`\`\`

Inline math: $a^2 + b^2 = c^2$

$$
\\int_0^\\infty e^{-x^2}\\,dx = \\frac{\\sqrt{\\pi}}{2}
$$

:::graph
digraph { A -> B -> C; }
:::
`;

export default class MarkdownTool extends PageComponent<{}, MarkdownToolState> {
  protected isPage = true;
  protected href = "markdown-tool";
  protected title = "Markdown Renderer";
  protected submenu = "tools";
  protected icon = "markdown";
  protected requiresAuth = true;

  private containerRef = React.createRef<HTMLDivElement>();

  constructor(props: {}) {
    super(props);
    this.state = { source: SAMPLE, sourceWidthPct: 35 };
  }

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
    window.removeEventListener("mousemove", this.onDragMove);
    window.removeEventListener("mouseup", this.onDragEnd);
  }

  private onDragStart = (e: React.MouseEvent) => {
    e.preventDefault();
    window.addEventListener("mousemove", this.onDragMove);
    window.addEventListener("mouseup", this.onDragEnd);
  };

  private onDragMove = (e: MouseEvent) => {
    const el = this.containerRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const pct = ((e.clientX - rect.left) / rect.width) * 100;
    this.setState({ sourceWidthPct: Math.min(MAX_PCT, Math.max(MIN_PCT, pct)) });
  };

  private onDragEnd = () => {
    window.removeEventListener("mousemove", this.onDragMove);
    window.removeEventListener("mouseup", this.onDragEnd);
  };

  render() {
    const { source, sourceWidthPct } = this.state;
    return (
      <div className="container-fluid pt-4">
        <h4 className="mb-3">{t("Markdown Renderer")}</h4>
        <div className="d-flex" ref={this.containerRef}>
          <div className="card" style={{ flex: `0 0 ${sourceWidthPct}%`, minWidth: 0 }}>
            <div className="card-body">
              <div className="text-muted small text-uppercase mb-2">{t("Source")}</div>
              {/* wrap="off" + white-space: pre: long lines scroll horizontally
                  inside the textarea's own box instead of reflowing narrower —
                  the box itself (border included) always matches the dragged width. */}
              <textarea
                className="form-control"
                wrap="off"
                style={{
                  width: "100%",
                  minHeight: 480,
                  resize: "vertical",
                  whiteSpace: "pre",
                  overflowX: "auto",
                  fontFamily: "monospace",
                  fontSize: "0.85rem",
                }}
                value={source}
                onChange={(e) => this.setState({ source: e.target.value })}
                spellCheck={false}
              />
            </div>
          </div>
          <div
            onMouseDown={this.onDragStart}
            title={t("Drag to resize")}
            style={{
              flex: "0 0 auto",
              width: 14,
              cursor: "col-resize",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <div style={{ width: 4, height: 40, borderRadius: 2, background: "#d7dcd9" }} />
          </div>
          <div className="card" style={{ flex: "1 1 auto", minWidth: 0 }}>
            <div className="card-body">
              <div className="text-muted small text-uppercase mb-2">{t("Preview")}</div>
              <MarkdownRender value={source} />
            </div>
          </div>
        </div>
      </div>
    );
  }
}
