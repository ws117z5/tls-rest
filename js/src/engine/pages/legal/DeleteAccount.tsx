import React, { useState } from "react";
import axios from "axios";
import { Link } from "react-router";
import PageComponent from "@engine/containers/PageComponent";

// Public "request account deletion" page. Posts to /api/deletion-request, which
// stores the request for an admin to action — nothing is deleted automatically.
// It is deliberately usable while logged out (a user who has lost access to
// their identity provider must still be able to ask).

const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

const REASONS = [
  "No longer using the site",
  "Privacy concern",
  "Created by mistake",
  "Other",
];

type Status = "idle" | "sending" | "sent" | "error";

const DeletionForm: React.FC = () => {
  const [email, setEmail] = useState("");
  const [reason, setReason] = useState("");
  const [note, setNote] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [website, setWebsite] = useState(""); // honeypot
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState("");

  const valid = EMAIL_RE.test(email.trim()) && confirmed;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid || status === "sending") return;
    setStatus("sending");
    setError("");
    try {
      await axios.post("/api/deletion-request", {
        email: email.trim(),
        reason,
        note: note.trim(),
        website,
      });
      setStatus("sent");
      setEmail("");
      setReason("");
      setNote("");
      setConfirmed(false);
    } catch {
      setStatus("error");
      setError(
        "Sorry — the request could not be submitted. Please try again later."
      );
    }
  };

  if (status === "sent") {
    return (
      <div className="alert alert-success" role="status">
        Your deletion request has been received. We will verify and process it,
        and confirm by email at the address you provided — normally within 30
        days.
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="mb-3">
        <label className="form-label" htmlFor="dr-email">
          Account email
        </label>
        <input
          id="dr-email"
          type="email"
          className="form-control"
          value={email}
          maxLength={200}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <div className="form-text">
          The email on the account you want deleted. We use it to locate the
          account and to confirm once it is done.
        </div>
      </div>

      <div className="mb-3">
        <label className="form-label" htmlFor="dr-reason">
          Reason <span className="text-muted">(optional)</span>
        </label>
        <select
          id="dr-reason"
          className="form-select"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        >
          <option value="">—</option>
          {REASONS.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
      </div>

      <div className="mb-3">
        <label className="form-label" htmlFor="dr-note">
          Anything else <span className="text-muted">(optional)</span>
        </label>
        <textarea
          id="dr-note"
          className="form-control"
          rows={4}
          value={note}
          maxLength={5000}
          onChange={(e) => setNote(e.target.value)}
        />
      </div>

      <div className="form-check mb-3">
        <input
          id="dr-confirm"
          type="checkbox"
          className="form-check-input"
          checked={confirmed}
          onChange={(e) => setConfirmed(e.target.checked)}
        />
        <label className="form-check-label" htmlFor="dr-confirm">
          I understand this permanently removes my account and personal data, and
          that publicly posted content may be kept in anonymized form.
        </label>
      </div>

      {/* Honeypot */}
      <div aria-hidden="true" style={{ position: "absolute", left: "-5000px" }}>
        <label htmlFor="dr-website">Website</label>
        <input
          id="dr-website"
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
        className="btn btn-danger"
        disabled={!valid || status === "sending"}
      >
        {status === "sending" ? "Submitting…" : "Request deletion"}
      </button>
    </form>
  );
};

export class DeleteAccountPage extends PageComponent<{}, {}> {
  protected href = "delete-account";
  protected title = "Delete Account";
  protected icon = "user-delete-request";
  protected isPage = true;
  protected submenu = "Legal";

  render() {
    return (
      <div className="container py-5" style={{ maxWidth: 720 }}>
        <h1 className="h2 mb-2">Request account deletion</h1>
        <p className="text-muted">
          Submit this form to have your account and personal data removed. You do
          not need to be signed in.
        </p>

        <div className="alert alert-warning">
          <strong>This cannot be undone.</strong> We delete your profile and
          personal identifiers. Content you posted publicly (e.g. posts and
          comments) may be retained in anonymized form unless you tell us
          otherwise in the note below.
        </div>

        <DeletionForm />

        <hr className="my-4" />
        <p className="small text-muted">
          See our <Link to="/pages/privacy">Privacy Policy</Link> for what we
          collect and how long we keep it. General questions go through the{" "}
          <Link to="/pages/contact">contact form</Link>.
        </p>
      </div>
    );
  }
}

export default DeleteAccountPage;
