import React, { useEffect, useState } from 'react';
import { Graphviz } from "@hpcc-js/wasm";

type Format = "svg" | "dot" | "json" | "dot_json" | "xdot_json" | "plain" | "plain-ext";
type Engine = "circo" | "dot" | "fdp" | "neato" | "osage" | "patchwork" | "twopi";
type GraphvizInstance = Awaited<ReturnType<typeof Graphviz.load>>;


async function renderGraph(dotSource: string, format: Format, engine: Engine): Promise<string> {
  // 1. Asynchronously load the WASM binary and initialize Graphviz
  const graphviz: GraphvizInstance = await Graphviz.load();

  // 2. Execute layout: layout(dotSource, outputFormat, layoutEngine)
  const svg: string = graphviz.layout(dotSource, format, engine);

  return svg;
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