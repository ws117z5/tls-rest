import React, { useEffect, useState } from "react";
import axios from "axios";
import { t, subscribe } from "@engine/controllers/i18n";
import FriendRequestActions from "@engine/modules/friends/FriendRequestActions";

// Friend-request control for another user's profile — rewritten against the
// real API (go/engine/modules/chats); the old /api/friends/* endpoints this
// used to call don't exist. Talks to:
//   GET  /api/chats                   -> find the direct chat with this user, if any
//   POST /api/chats/direct/{userId}   -> send a request, optionally with a message
//   POST /api/chats/{id}/accept       -> accept an incoming request
//   POST /api/chats/{id}/unfriend     -> end an existing friendship

interface ChatMember {
  id: number;
  status: string;
  message?: string;
}
interface ChatDto {
  id: number;
  type: string;
  myStatus: string;
  members: ChatMember[];
}

type Status = "loading" | "none" | "pending_sent" | "pending_received" | "friends";

interface Props {
  userId: number | string;
}

const FriendButton: React.FC<Props> = ({ userId }) => {
  const [status, setStatus] = useState<Status>("loading");
  const [chatId, setChatId] = useState<number | null>(null);
  const [theirMessage, setTheirMessage] = useState<string>("");
  const [composing, setComposing] = useState(false);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [, forceRender] = useState(0);

  useEffect(() => subscribe(() => forceRender((n) => n + 1)), []);

  const load = () => {
    axios
      .get("/api/chats")
      .then((res) => {
        const chats: ChatDto[] = res.data?.chats || [];
        const chat = chats.find(
          (c) => c.type === "direct" && c.members.some((m) => String(m.id) === String(userId)),
        );
        if (!chat) {
          setStatus("none");
          return;
        }
        const other = chat.members.find((m) => String(m.id) === String(userId));
        const mine = chat.members.find((m) => String(m.id) !== String(userId));
        setChatId(chat.id);
        if (chat.myStatus === "pending") {
          setStatus("pending_received");
          setTheirMessage(mine?.message || "");
        } else if (chat.myStatus === "accepted" && other?.status === "pending") {
          setStatus("pending_sent");
        } else if (chat.myStatus === "accepted" && other?.status === "accepted") {
          setStatus("friends");
        } else {
          setStatus("none");
        }
      })
      .catch(() => setStatus("none"));
  };

  useEffect(load, [userId]);

  const sendRequest = () => {
    if (busy) return;
    setBusy(true);
    axios
      .post(`/api/chats/direct/${userId}`, { message: draft.trim() })
      .then(() => {
        setComposing(false);
        setDraft("");
        load();
      })
      .catch(() => {})
      .finally(() => setBusy(false));
  };

  const unfriend = () => {
    if (busy || !chatId) return;
    setBusy(true);
    axios.post(`/api/chats/${chatId}/unfriend`).then(load).catch(() => {}).finally(() => setBusy(false));
  };

  if (status === "loading") return null;

  if (status === "friends") {
    return (
      <button type="button" className="btn btn-sm btn-outline-secondary" disabled={busy} onClick={unfriend}>
        {t("Friends")} · {t("Unfriend")}
      </button>
    );
  }

  if (status === "pending_sent") {
    return (
      <button type="button" className="btn btn-sm btn-outline-secondary" disabled>
        {t("Request sent")}
      </button>
    );
  }

  if (status === "pending_received" && chatId) {
    return (
      <div>
        {theirMessage && <div className="text-muted small mb-2">&ldquo;{theirMessage}&rdquo;</div>}
        <FriendRequestActions chatId={chatId} userId={userId} onResolved={load} />
      </div>
    );
  }

  if (composing) {
    return (
      <div className="d-flex gap-2 align-items-start">
        <textarea
          className="form-control form-control-sm"
          style={{ width: 240 }}
          rows={2}
          placeholder={t("Add a message (optional)")}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <button type="button" className="btn btn-sm btn-primary" disabled={busy} onClick={sendRequest}>
          {t("Send")}
        </button>
      </div>
    );
  }

  return (
    <button type="button" className="btn btn-sm btn-primary" disabled={busy} onClick={() => setComposing(true)}>
      {t("Add Friend")}
    </button>
  );
};

export default FriendButton;
