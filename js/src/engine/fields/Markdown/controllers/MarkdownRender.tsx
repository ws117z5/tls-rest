import React from "react";
import Markdown, { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import remarkDirective from "remark-directive";
import { safeUrl, resolveImageSrc } from "@engine/fields/Markdown/controllers/markdown";
import allowedComponents, { allowedNames } from "../components";

// Shared markdown renderer used by both the view and the editor preview.
//
// react-markdown is safe by default: it does not render raw HTML embedded in the
// source, so user content cannot inject markup — there is no dangerouslySet-
// InnerHTML anywhere here. The pipeline adds:
//   - remark-gfm      GitHub extras (tables, task lists, strikethrough, autolinks)
//   - remark-math +   inline $a^2+b^2$ and block $$…$$ LaTeX, rendered by KaTeX
//     rehype-katex     (rehype-katex transforms math nodes in the tree — it does
//                       not enable raw HTML, so safety is preserved)
//   - remark-directive :::name / :name[…] directives, resolved against a fixed
//                       allow-list of components (see components/registry).
//
// This is "MDX-flavoured markdown", not true MDX: content can invoke ONLY the
// vetted components in the components folder, and only with string props. It can
// never evaluate arbitrary JSX/components, which would be code execution on
// stored content.

// Turn allowed directive nodes into elements named after the directive (so the
// components map below renders them). Unknown directives degrade to a neutral
// element, so their inner text still shows but no component is invoked.
function remarkAllowedDirectives(allowed: Set<string>) {
    return () => (tree: any) => {
        const visit = (node: any) => {
            if (
                node &&
                (node.type === "textDirective" ||
                    node.type === "leafDirective" ||
                    node.type === "containerDirective")
            ) {
                const data = node.data || (node.data = {});
                if (allowed.has(node.name)) {
                    data.hName = node.name;
                    data.hProperties = { ...(node.attributes || {}) };
                } else {
                    data.hName = node.type === "containerDirective" ? "div" : "span";
                }
            }
            if (node && Array.isArray(node.children)) node.children.forEach(visit);
        };
        visit(tree);
    };
}

const baseComponents: Components = {
    a({ node, href, children, ...props }) {
        const safe = safeUrl(href || "");
        if (!safe) return <>{children}</>;
        return (
            <a href={safe} rel="noopener noreferrer" target="_blank" {...props}>
                {children}
            </a>
        );
    },
    img({ node, src, alt, ...props }) {
        const safe = resolveImageSrc(typeof src === "string" ? src : "");
        if (!safe) return null;
        return <img src={safe} alt={alt || ""} loading="lazy" className="md-img" {...props} />;
    },
};

// Base renderers plus the whitelisted content components (keyed by directive
// name). Cast because the component map keys are custom directive names, not
// standard HTML tags.
const components = { ...baseComponents, ...allowedComponents } as unknown as Components;

interface MarkdownRenderProps {
    value?: string;
    className?: string;
}

const MarkdownRender: React.FC<MarkdownRenderProps> = ({ value, className }) => {
    // KaTeX (rehype-katex + its CSS) is heavy, so it's code-split and loaded on
    // demand. Until it resolves, math renders as plain text; once loaded, the
    // component re-renders with the plugin applied. The plugin is only added to
    // the pipeline after it has loaded (never used before it's ready).
    const [katexPlugin, setKatexPlugin] = React.useState<any>(null);

    React.useEffect(() => {
        let mounted = true;
        Promise.all([
            import("rehype-katex"),
            // side-effect: pulls in katex's stylesheet as its own chunk
            import("katex/dist/katex.min.css"),
        ])
            .then(([mod]) => {
                if (mounted) setKatexPlugin(() => mod.default);
            })
            .catch(() => {
                /* leave math as plain text if katex fails to load */
            });
        return () => {
            mounted = false;
        };
    }, []);

    return (
        <div className={`markdown-body ${className || ""}`}>
            <Markdown
                remarkPlugins={[
                    remarkGfm,
                    remarkMath,
                    remarkDirective,
                    remarkAllowedDirectives(allowedNames),
                ]}
                rehypePlugins={katexPlugin ? [katexPlugin] : []}
                components={components}
            >
                {value || ""}
            </Markdown>
        </div>
    );
};

export default MarkdownRender;