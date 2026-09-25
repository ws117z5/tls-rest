import React, { useEffect, useState } from "react";
import { Link } from "react-router";
import axios from "axios";
import { t, subscribe } from "@engine/controllers/i18n";

// The list of conversations for the signed-in user, most recent first. Used
// both by the standalone Messages page and embedded in the user's own
// profile page. Opening one goes to that user's public profile, where the
// actual thread (MessageThread) is embedded.

interface Conversation {
  userId: number;
  userName: string;
  image: string;
  lastBody: string;
  lastCreated: string;
  unread: number;
}

function whenText(v: string): string {
  if (!v) return "";
  const d = new Date(v);
  return isNaN(d.getTime()) ? String(v) : d.toLocaleString();
}

const ConversationList: React.FC = () => {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [, forceRender] = useState(0);

  useEffect(() => {
    return subscribe(() => forceRender((n) => n + 1));
  }, []);

  useEffect(() => {
    let cancelled = false;
    axios
      .get("/api/messages/inbox")
      .then((res) => {
        if (cancelled) return;
        const data = (res.data && res.data.conversations) || [];
        setConversations(Array.isArray(data) ? data : []);
        setLoading(false);
      })
      .catch(() => {
        if (cancelled) return;
        setError(t("Could not load messages."));
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) return <p className="text-muted small">{t("Loading…")}</p>;
  if (error) return <div className="alert alert-warning py-2">{error}</div>;
  if (conversations.length === 0) return <p className="text-muted small">{t("No conversations yet.")}</p>;

  return (
    <ul className="list-group">
      {conversations.map((c) => (
        <Link
          key={c.userId}
          to={`/u/${c.userId}`}
          className="list-group-item list-group-item-action d-flex justify-content-between align-items-center"
        >
          <div className="d-flex align-items-center gap-2">
            {c.image ? (
              <img
                src={c.image}
                alt=""
                style={{ width: 36, height: 36, borderRadius: "50%", objectFit: "cover" }}
              />
            ) : null}
            <div>
              <div className="fw-semibold">{c.userName}</div>
              <div className="text-muted small text-truncate" style={{ maxWidth: 420 }}>
                {c.lastBody}
              </div>
            </div>
          </div>
          <div className="text-end">
            <div className="text-muted small">{whenText(c.lastCreated)}</div>
            {c.unread > 0 && <span className="badge bg-success">{c.unread}</span>}
          </div>
        </Link>
      ))}
    </ul>
  );
};

export default ConversationList;
