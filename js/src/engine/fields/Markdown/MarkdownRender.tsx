import React from "react";
import Markdown, { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { safeUrl, resolveImageSrc } from "@controllers/markdown";

// Shared markdown renderer used by both the view and the editor preview.
//
// react-markdown is safe by default: it does not render raw HTML embedded in the
// source, so user content cannot inject markup — there is no dangerouslySet-
// InnerHTML anywhere here. remark-gfm adds GitHub-flavoured extras (tables,
// task lists, strikethrough, autolinks).
//
// We override links and images to enforce safe targets and to resolve bare
// backend image ids (![alt](<id>)) to their /api/posts/images/<id> URL.
//
// Note: this is "MDX-flavoured markdown", not true MDX — we deliberately do NOT
// evaluate arbitrary JSX/components from stored content, since executing user-
// authored code would be a serious security hole. To add math, install
// remark-math + rehype-katex and pass them below (katex CSS required).

const components: Components = {
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

interface MarkdownRenderProps {
    value?: string;
    className?: string;
}

const MarkdownRender: React.FC<MarkdownRenderProps> = ({ value, className }) => (
    <div className={`markdown-body ${className || ""}`}>
        <Markdown remarkPlugins={[remarkGfm]} components={components}>
            {value || ""}
        </Markdown>
    </div>
);

export default MarkdownRender;