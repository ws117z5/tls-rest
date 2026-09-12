import React, { useState } from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import useT from "@engine/useT";
import { t, subscribe } from "@engine/i18n";

// Contact form. Posts to /api/contact, which stores the message server-side —
// no email address is exposed anywhere on the site.

const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

type Status = "idle" | "sending" | "sent" | "error";

const ContactForm: React.FC = () => {
  const t = useT();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [subject, setSubject] = useState("");
  const [message, setMessage] = useState("");
  const [website, setWebsite] = useState(""); // honeypot
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState("");

  const valid =
    name.trim().length > 0 &&
    EMAIL_RE.test(email.trim()) &&
    message.trim().length > 0;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid || status === "sending") return;
    setStatus("sending");
    setError("");
    try {
      await axios.post("/api/contact", {
        name: name.trim(),
        email: email.trim(),
        subject: subject.trim(),
        message: message.trim(),
        website,
      });
      setStatus("sent");
      setName("");
      setEmail("");
      setSubject("");
      setMessage("");
    } catch {
      setStatus("error");
      setError(t("Sorry — the message could not be sent. Please try again later."));
    }
  };

  if (status === "sent") {
    return (
      <div className="alert alert-success" role="status">
        {t("Thanks — your message has been received. We'll get back to you at the address you provided.")}
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="mb-3">
        <label className="form-label" htmlFor="cf-name">
          {t("Name")}
        </label>
        <input
          id="cf-name"
          className="form-control"
          value={name}
          maxLength={120}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>

      <div className="mb-3">
        <label className="form-label" htmlFor="cf-email">
          {t("Email")}
        </label>
        <input
          id="cf-email"
          type="email"
          className="form-control"
          value={email}
          maxLength={200}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <div className="form-text">{t("So we can reply to you.")}</div>
      </div>

      <div className="mb-3">
        <label className="form-label" htmlFor="cf-subject">
          {t("Subject")} <span className="text-muted">({t("optional")})</span>
        </label>
        <input
          id="cf-subject"
          className="form-control"
          value={subject}
          maxLength={200}
          onChange={(e) => setSubject(e.target.value)}
        />
      </div>

      <div className="mb-3">
        <label className="form-label" htmlFor="cf-message">
          {t("Message")}
        </label>
        <textarea
          id="cf-message"
          className="form-control"
          rows={6}
          value={message}
          maxLength={5000}
          onChange={(e) => setMessage(e.target.value)}
          required
        />
      </div>

      {/* Honeypot: hidden from users, ignored by the server when filled. */}
      <div aria-hidden="true" style={{ position: "absolute", left: "-5000px" }}>
        <label htmlFor="cf-website">Website</label>
        <input
          id="cf-website"
          tabIndex={-1}
          autoComplete="off"
          value={website}
          onChange={(e) => setWebsite(e.target.value)}
        />
      </div>

      {status === "error" && (
        <div className="alert alert-danger py-2">{error}</div>
      )}

      <button
        type="submit"
        className="btn btn-primary"
        disabled={!valid || status === "sending"}
      >
        {status === "sending" ? t("Sending…") : t("Send message")}
      </button>
    </form>
  );
};

export class ContactPage extends PageComponent<{}, {}> {
  protected href = "contact";
  protected title = "Contact";
  protected icon = "messages-sm";
  protected isPage = true;
  protected submenu = "Legal";
  private unsubscribeI18n?: () => void;

  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  render() {
    return (
      <div className="container py-5" style={{ maxWidth: 720 }}>
        <h1 className="h2 mb-2">{t("Contact")}</h1>
        <p className="text-muted mb-4">
          {t("Use the form below to get in touch. Your message is delivered privately — no email address is published on this site.")}
        </p>
        <ContactForm />
      </div>
    );
  }
}

export default ContactPage;
