import React from "react";
import { Link, useNavigate } from "react-router";
import PageComponent from "@engine/containers/PageComponent";
import HtmlView from "@engine/fields/Html/View";
import { useHtml } from "@engine/controllers/html";

// Terms of Service and Privacy Policy pages. The text lives in the html module
// (rows published as legal.terms / legal.privacy) and is fetched from the server;
// these pages only frame it. PageComponent's own data fetch stays off, and the
// SPA shell serves /pages/terms and /pages/privacy directly.

const shellStyle: React.CSSProperties = { maxWidth: 820 };

const LegalDocument: React.FC<{ slug: string; title: string }> = ({ slug, title }) => {
  const navigate = useNavigate();
  const { html, failed } = useHtml(slug);

  // Links inside the stored HTML point at in-app paths; follow them client-side instead of reloading the shell.
  const followLinks = (e: React.MouseEvent) => {
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (a && href && href.startsWith("/") && !e.metaKey && !e.ctrlKey && !e.shiftKey) {
      e.preventDefault();
      navigate(href);
    }
  };

  return (
    <div className="container py-5" style={shellStyle}>
      <h1 className="h2 mb-3">{title}</h1>
      {failed && <p className="text-muted">This document could not be loaded.</p>}
      <div className="legal-body" onClick={followLinks}>
        <HtmlView value={html} />
      </div>
      <hr className="my-4" />
      <p className="small text-muted">
        Questions about this document? Reach us through the <Link to="/pages/contact">contact form</Link>. See also our{" "}
        <Link to="/pages/terms">Terms of Service</Link> and <Link to="/pages/privacy">Privacy Policy</Link>.
      </p>
    </div>
  );
};

export class TermsPage extends PageComponent<{}, {}> {
  protected href = "terms";
  protected title = "Terms of Service";
  protected icon = "terms";
  protected isPage = true;
  protected submenu = "Legal";

  render() {
    return <LegalDocument slug="legal.terms" title="Terms of Service" />;
  }
}

export class PrivacyPage extends PageComponent<{}, {}> {
  protected href = "privacy";
  protected title = "Privacy Policy";
  protected icon = "privacy";
  protected isPage = true;
  protected submenu = "Legal";

  render() {
    return <LegalDocument slug="legal.privacy" title="Privacy Policy" />;
  }
}
