import React, { useEffect, useState } from "react";
import AppConfig from "@engine/controllers/Appconfig";

// Fenced code blocks (```go, ```ts, ```js, …) get real syntax highlighting via
// Prism (through react-syntax-highlighter), themed as VS Code Dark+/Light+ to
// match AppConfig.theme(). The highlighter core, languages, and theme are
// heavy, so they're only pulled in (once per theme, cached) the first time a
// fenced block actually renders.

type HighlighterType = React.ComponentType<any> & {
  registerLanguage: (name: string, lang: unknown) => void;
};

interface Loaded {
  Highlighter: HighlighterType;
  style: Record<string, React.CSSProperties>;
}

const loadPromises = new Map<string, Promise<Loaded>>();

function load(theme: string): Promise<Loaded> {
  let p = loadPromises.get(theme);
  if (!p) {
    const styleImport =
      theme === "dark"
        ? import("react-syntax-highlighter/dist/esm/styles/prism/vsc-dark-plus")
        : import("react-syntax-highlighter/dist/esm/styles/prism/vs");
    p = Promise.all([
      import("react-syntax-highlighter/dist/esm/prism-light"),
      styleImport,
      import("react-syntax-highlighter/dist/esm/languages/prism/go"),
      import("react-syntax-highlighter/dist/esm/languages/prism/typescript"),
      import("react-syntax-highlighter/dist/esm/languages/prism/tsx"),
      import("react-syntax-highlighter/dist/esm/languages/prism/javascript"),
      import("react-syntax-highlighter/dist/esm/languages/prism/jsx"),
      import("react-syntax-highlighter/dist/esm/languages/prism/json"),
      import("react-syntax-highlighter/dist/esm/languages/prism/bash"),
      import("react-syntax-highlighter/dist/esm/languages/prism/sql"),
      import("react-syntax-highlighter/dist/esm/languages/prism/yaml"),
      import("react-syntax-highlighter/dist/esm/languages/prism/markup"),
    ]).then(
      ([
        highlighterMod,
        styleMod,
        go,
        typescript,
        tsx,
        javascript,
        jsx,
        json,
        bash,
        sql,
        yaml,
        markup,
      ]) => {
        const Highlighter = highlighterMod.default as HighlighterType;
        Highlighter.registerLanguage("go", go.default);
        Highlighter.registerLanguage("typescript", typescript.default);
        Highlighter.registerLanguage("ts", typescript.default);
        Highlighter.registerLanguage("tsx", tsx.default);
        Highlighter.registerLanguage("javascript", javascript.default);
        Highlighter.registerLanguage("js", javascript.default);
        Highlighter.registerLanguage("jsx", jsx.default);
        Highlighter.registerLanguage("json", json.default);
        Highlighter.registerLanguage("bash", bash.default);
        Highlighter.registerLanguage("sh", bash.default);
        Highlighter.registerLanguage("sql", sql.default);
        Highlighter.registerLanguage("yaml", yaml.default);
        Highlighter.registerLanguage("html", markup.default);
        Highlighter.registerLanguage("xml", markup.default);
        return { Highlighter, style: styleMod.default as Record<string, React.CSSProperties> };
      }
    );
    loadPromises.set(theme, p);
  }
  return p;
}

interface CodeBlockProps {
  className?: string;
  children?: React.ReactNode;
}

// react-markdown's `code` renderer. A fenced block carries a "language-xxx"
// className (set by remark from the ```lang fence); an inline `code` span
// never does — the only reliable way to tell them apart in react-markdown v9+,
// which dropped the old `inline` prop.
const CodeBlock: React.FC<CodeBlockProps> = ({ className, children }) => {
  const match = /language-(\w+)/.exec(className || "");
  const [loaded, setLoaded] = useState<Loaded | null>(null);

  const theme = AppConfig.theme();

  useEffect(() => {
    if (!match) return;
    let mounted = true;
    load(theme).then((result) => {
      if (mounted) setLoaded(result);
    });
    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [!!match, theme]);

  if (!match) {
    return <code className={className}>{children}</code>;
  }

  const code = String(children).replace(/\n$/, "");

  if (!loaded) {
    // Unhighlighted fallback until the highlighter resolves — mirrors how
    // MarkdownRender renders math as plain text before KaTeX loads.
    return (
      <pre>
        <code className={className}>{code}</code>
      </pre>
    );
  }

  const { Highlighter, style } = loaded;
  return (
    <Highlighter
      language={match[1]}
      style={style}
      customStyle={{ margin: "0.5em 0", borderRadius: 6, fontSize: "0.85em" }}
    >
      {code}
    </Highlighter>
  );
};

export default CodeBlock;
