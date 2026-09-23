import React, { useEffect, useState } from 'react';

type Format = "svg" | "dot" | "json" | "dot_json" | "xdot_json" | "plain" | "plain-ext";
type Engine = "circo" | "dot" | "fdp" | "neato" | "osage" | "patchwork" | "twopi";

const RENDER_SCALE = 0.8;

// Shrinks the SVG root's width/height (leaving viewBox untouched, which makes
// the browser scale the drawing to fit the smaller box).
function scaleSvg(svg: string, factor: number): string {
  return svg.replace(
    /(<svg[^>]*\bwidth=")([\d.]+)(pt|px)?("[^>]*\bheight=")([\d.]+)(pt|px)?(")/,
    (_m, pre, w, wUnit = "", mid, h, hUnit = "", post) =>
      `${pre}${(parseFloat(w) * factor).toFixed(2)}${wUnit}${mid}${(parseFloat(h) * factor).toFixed(2)}${hUnit}${post}`
  );
}

// The graphviz WASM is heavy, so it's code-split and loaded on demand — only when
// a graph is actually rendered — and awaited before use.
async function renderGraph(dotSource: string, format: Format, engine: Engine): Promise<string> {
  const { Graphviz } = await import("@hpcc-js/wasm-graphviz");
  const graphviz = await Graphviz.load();
  const svg: string = graphviz.layout(dotSource, format, engine);
  return scaleSvg(svg, RENDER_SCALE);
};

interface DotGraphProps {
  dotSource: string;
  outputForma?: Format;
  engine?: Engine;
  className?: string;
}

const DotGraph: React.FC<DotGraphProps> = ({
  dotSource,
  engine = "dot",
  className = "",
}) => {
  const [svgContent, setSvgContent] = useState<string>("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;

    async function runLayout() {
      try {
        setError(null);
        // Load WASM instance
        const svg = await renderGraph(dotSource, "svg", engine);

        if (isMounted) {
          setSvgContent(svg);
        }
      } catch (err: unknown) {
        if (isMounted) {
          const msg = err instanceof Error ? err.message : "Error parsing DOT syntax";
          setError(msg);
        }
      }
    }

    if (dotSource.trim()) {
      runLayout();
    } else {
      setSvgContent("");
    }

    return () => {
      isMounted = false;
    };
  }, [dotSource, engine]);

  if (error) {
    return (
      <div style={{ color: "red", padding: "10px", border: "1px solid red" }}>
        <strong>DOT Error:</strong> {error}
      </div>
    );
  }

  // Guaranteed JSX Return Path 2: Normal State
  return (
    <div
      className={className}
      dangerouslySetInnerHTML={{ __html: svgContent }}
    />
  );
};

interface GraphProps {
    children?: React.ReactNode;
}

const Graph: React.FC<GraphProps> = ({ children }) => {

  // Helper function to recursively extract text from nested React Nodes
const extractText = (node: React.ReactNode): string => {
  if (!node) return "";
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map(extractText).join("");
  }
  if (React.isValidElement(node) && node.props) {
    // Explicitly cast the unknown props.children to React.ReactNode
    const children = (node.props as { children?: React.ReactNode }).children;
    return extractText(children);
  }
  return "";
};

  const str = extractText(children)

    return (
        <div className={`markdown-graph my-2`} role="graph">
            <DotGraph dotSource={str} />
        </div>
    );
};

export default Graph;