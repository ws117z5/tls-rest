import React from "react";
import { Link } from "react-router";
import PageComponent from "@engine/containers/PageComponent";

// Static Terms of Service and Privacy Policy pages. Pure frontend — no backend
// endpoint — so PageComponent's data fetch stays off (loadsData = false) and the
// SPA shell serves /pages/terms and /pages/privacy directly.

const EFFECTIVE_DATE = "September 10, 2026";
const SITE = "koroteev.dev";
const OAUTH_PROVIDERS = "Google, Facebook, X, and GitHub";

// All contact routes through the on-site form — no address is published here.
const ContactLink: React.FC = () => (
  <Link to="/pages/contact">contact form</Link>
);

const shellStyle: React.CSSProperties = { maxWidth: 820 };

const LegalShell: React.FC<{ title: string; children: React.ReactNode }> = ({
  title,
  children,
}) => (
  <div className="container py-5" style={shellStyle}>
    <h1 className="h2 mb-1">{title}</h1>
    <p className="text-muted small mb-4">Last updated: {EFFECTIVE_DATE}</p>
    <div className="legal-body">{children}</div>
    <hr className="my-4" />
    <p className="small text-muted">
      Questions about this document? Reach us through the <ContactLink />. See
      also our <Link to="/pages/terms">Terms of Service</Link> and{" "}
      <Link to="/pages/privacy">Privacy Policy</Link>.
    </p>
  </div>
);

const H: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <h2 className="h5 mt-4 mb-2">{children}</h2>
);

// --- Terms of Service ---------------------------------------------------------

export class TermsPage extends PageComponent<{}, {}> {
  protected href = "terms";
  protected title = "Terms of Service";
  protected icon = "terms";
  protected isPage = true;
  protected submenu = "Legal";

  render() {
    return (
      <LegalShell title="Terms of Service">
        <p>
          {SITE} is a personal project operated by Vladimir Koroteev ("we", "us").
          By accessing the site or creating an account you agree to these Terms. If
          you do not agree, do not use the site.
        </p>

        <H>1. Accounts</H>
        <p>
          Some features require an account, which you create by signing in with a
          third-party identity provider ({OAUTH_PROVIDERS}). You are responsible
          for activity under your account and for keeping your provider
          credentials secure. You must be at least 18 years old, or the age of
          digital consent in your jurisdiction, to hold an account.
        </p>

        <H>2. Your content</H>
        <p>
          You may submit content such as posts, comments, and uploaded images
          ("Your Content"). You retain ownership of Your Content. You grant us a
          non-exclusive, worldwide, royalty-free license to store, display, and
          process it solely to operate and provide the site. You are responsible
          for Your Content and confirm you have the rights to submit it.
        </p>

        <H>3. Acceptable use</H>
        <p>You agree not to:</p>
        <ul>
          <li>post unlawful, infringing, hateful, or harassing material;</li>
          <li>upload malware or attempt to breach, probe, or disrupt the service;</li>
          <li>
            access the site through automated means beyond ordinary browsing, or
            scrape it at a rate that degrades service for others;
          </li>
          <li>impersonate others or misrepresent your affiliation.</li>
        </ul>
        <p>
          We may remove content or suspend accounts that violate these Terms, at
          our discretion and without notice.
        </p>

        <H>4. Intellectual property</H>
        <p>
          The site's software, design, and original text are owned by us and
          provided for your personal, non-commercial use. Nothing here transfers
          any right in our marks or code except as expressly stated.
        </p>

        <H>5. Third-party services</H>
        <p>
          Sign-in relies on third-party identity providers ({OAUTH_PROVIDERS}).
          Your use of those services is governed by each provider's own terms and
          privacy policy. We are not responsible for third-party services.
        </p>

        <H>6. Disclaimer</H>
        <p>
          The site is provided "as is" and "as available", without warranties of
          any kind, express or implied, including merchantability, fitness for a
          particular purpose, and non-infringement. We do not warrant that the site
          will be uninterrupted, secure, or error-free, or that content will be
          preserved.
        </p>

        <H>7. Limitation of liability</H>
        <p>
          The site is provided free of charge. To the maximum extent permitted by
          law, we will not be liable for any indirect, incidental, special,
          consequential, or punitive damages, or for lost data, profits, or
          goodwill, arising from or related to your use of the site — whether based
          on warranty, contract, tort, or any other legal theory.
        </p>

        <H>8. Termination</H>
        <p>
          You may stop using the site at any time. We may suspend or terminate
          access at any time. Sections that by their nature should survive
          termination (ownership, disclaimers, limitation of liability) will
          survive.
        </p>

        <H>9. Changes</H>
        <p>
          We may update these Terms. Material changes will be reflected by the
          "Last updated" date above. Continued use after a change means you accept
          the revised Terms.
        </p>

        <H>10. Contact</H>
        <p>
          For questions about these Terms, reach us through the <ContactLink />.
        </p>
      </LegalShell>
    );
  }
}

// --- Privacy Policy ---------------------------------------------------------

export class PrivacyPage extends PageComponent<{}, {}> {
  protected href = "privacy";
  protected title = "Privacy Policy";
  protected icon = "privacy";
  protected isPage = true;
  protected submenu = "Legal";

  render() {
    return (
      <LegalShell title="Privacy Policy">
        <p>
          This policy explains what {SITE} collects, why, and what choices you
          have. {SITE} is a personal project; data is kept to the minimum needed to
          run it.
        </p>

        <H>Information we collect</H>
        <ul>
          <li>
            <strong>Account information.</strong> When you sign in through a
            third-party identity provider — {OAUTH_PROVIDERS} — we receive basic
            profile information from that provider, typically your name, email
            address, and profile image. The exact fields depend on the provider
            and the permissions you grant it. We use these to create your account
            and identify you on the site.
          </li>
          <li>
            <strong>Content you submit.</strong> Posts, comments, group and profile
            settings, and any images you upload, along with timestamps and the
            account that created them.
          </li>
          <li>
            <strong>Technical and log data.</strong> IP address, user-agent,
            requested URL, response status, and timing are recorded in an access
            log for security, debugging, and abuse prevention.
          </li>
          <li>
            <strong>Cookies.</strong> A session cookie keeps you signed in. It is
            required for authenticated features and is not used for advertising.
          </li>
        </ul>

        <H>How we use information</H>
        <ul>
          <li>to operate the site and provide the features you request;</li>
          <li>to authenticate you and maintain your session;</li>
          <li>
            to secure the service — detect and block abuse, investigate incidents,
            and enforce our Terms;
          </li>
          <li>to diagnose and fix problems.</li>
        </ul>
        <p>
          We do not sell your personal information and we do not use it for
          third-party advertising.
        </p>

        <H>Third parties</H>
        <p>
          <strong>Identity providers.</strong> Sign-in is offered through{" "}
          {OAUTH_PROVIDERS}. When you choose one, that provider authenticates you
          and returns the profile data described above; your interaction with it is
          governed by its own privacy policy. We only request basic profile and
          email scope.
        </p>
        <p>
          <strong>Infrastructure.</strong> Our server may sit behind standard
          providers (hosting, TLS termination, CDN) that process requests in
          transit. These parties process data on our behalf to deliver the
          service.
        </p>

        <H>Data retention</H>
        <p>
          Account and content data is kept while your account is active. Access-log
          entries are retained for a limited period for security and then aged out.
          When you ask us to delete your account, we remove your profile and
          personal identifiers; content you posted publicly may be retained in
          anonymized form unless you ask otherwise.
        </p>

        <H>Your choices and rights</H>
        <ul>
          <li>
            <strong>Access and correction.</strong> You can view and edit your
            profile from your account. For a copy of other data we hold about you,
            contact us.
          </li>
          <li>
            <strong>Deletion.</strong> Use the{" "}
            <Link to="/pages/delete-account">account deletion request</Link> page
            to have your account and associated personal data removed. You do not
            need to be signed in.
          </li>
          <li>
            <strong>Cookies.</strong> You can clear or block cookies in your
            browser, but authenticated features will not work without the session
            cookie.
          </li>
        </ul>
        <p>
          Depending on where you live, you may have additional rights under laws
          such as the GDPR or CCPA (to object, restrict, or port your data). Reach
          out and we will honor applicable requests.
        </p>

        <H>Security</H>
        <p>
          Connections are served over TLS and passwords for local accounts, where
          used, are stored hashed. No method of transmission or storage is
          completely secure; we cannot guarantee absolute security.
        </p>

        <H>Children</H>
        <p>
          The site is not directed to children under 13, and we do not knowingly
          collect their personal information.
        </p>

        <H>Changes</H>
        <p>
          We may update this policy. Material changes will be reflected by the
          "Last updated" date above.
        </p>

        <H>Contact</H>
        <p>
          For privacy questions or requests, reach us through the <ContactLink />.
        </p>
      </LegalShell>
    );
  }
}
