import React, { Component } from "react";
import axios from "axios";
import { t, subscribe } from "@engine/i18n";
import Auth from "@controllers/auth";

// A direct-message thread with one other user. Talks to:
//   GET  /api/messages/thread/{userId}  -> { messages: [...] } (also marks read)
//   POST /api/messages/thread/{userId}  -> { id }
// Kept plain text (no Markdown) — this is private messaging, not published
// content like comments.

interface MessageItem {
  id: number;
  body: string;
  created: string;
  mine: boolean;
}

interface Props {
  otherUserId: number | string;
}

interface State {
  messages: MessageItem[];
  loading: boolean;
  error: string;
  body: string;
  sending: boolean;
}

function whenText(v: string): string {
  if (!v) return "";
  const d = new Date(v);
  return isNaN(d.getTime()) ? String(v) : d.toLocaleString();
}

class MessageThread extends Component<Props, State> {
  state: State = { messages: [], loading: false, error: "", body: "", sending: false };
  private unsubscribeI18n?: () => void;

  componentDidMount() {
    this.load();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }

  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  componentDidUpdate(prev: Props) {
    if (prev.otherUserId !== this.props.otherUserId) this.load();
  }

  private load = async () => {
    this.setState({ loading: true, error: "" });
    try {
      const res = await axios.get(`/api/messages/thread/${this.props.otherUserId}`);
      const messages = (res.data && res.data.messages) || [];
      this.setState({ messages: Array.isArray(messages) ? messages : [], loading: false });
    } catch {
      this.setState({ error: t("Could not load messages."), loading: false });
    }
  };

  private send = async () => {
    const text = this.state.body.trim();
    if (!text || this.state.sending) return;
    this.setState({ sending: true });
    try {
      await axios.post(`/api/messages/thread/${this.props.otherUserId}`, { body: text });
      this.setState({ body: "", sending: false });
      await this.load();
    } catch {
      this.setState({ sending: false, error: t("Could not send message.") });
    }
  };

  render() {
    if (!Auth.isAuthenticated()) {
      return <p className="text-muted small">{t("Sign in to send a message.")}</p>;
    }

    const { messages, loading, error, body, sending } = this.state;

    return (
      <section className="message-thread">
        {error && <div className="alert alert-warning py-2">{error}</div>}

        {loading && messages.length === 0 ? (
          <p className="text-muted small">{t("Loading…")}</p>
        ) : messages.length === 0 ? (
          <p className="text-muted small">{t("No messages yet.")}</p>
        ) : (
          <ul className="list-unstyled mb-3">
            {messages.map((m) => {
              const mine = m.mine;
              return (
                <li
                  key={m.id}
                  className={`d-flex mb-2 ${mine ? "justify-content-end" : "justify-content-start"}`}
                >
                  <div
                    className="p-2 rounded"
                    style={{
                      maxWidth: "75%",
                      background: mine ? "rgba(111, 242, 160, 0.25)" : "#f1f3f2",
                    }}
                  >
                    <div>{m.body}</div>
                    <div className="text-muted small mt-1">{whenText(m.created)}</div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        <div className="d-flex gap-2">
          <input
            className="form-control"
            value={body}
            placeholder={t("Write a message…")}
            onChange={(e) => this.setState({ body: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") this.send();
            }}
          />
          <button
            type="button"
            className="btn btn-primary"
            disabled={!body.trim() || sending}
            onClick={this.send}
          >
            {t("Send")}
          </button>
        </div>
      </section>
    );
  }
}

export default MessageThread;
