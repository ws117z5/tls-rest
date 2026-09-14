import React, { useEffect, useState } from "react";
import axios from "axios";
import { t, subscribe } from "@engine/i18n";

// Friend-request control for another user's profile. Talks to:
//   GET  /api/friends/status/{id}   -> { status }
//   POST /api/friends/request/{id}  -> send a request
//   POST /api/friends/accept/{id}   -> accept an incoming request
//   POST /api/friends/remove/{id}   -> unfriend / cancel / decline

type Status = "none" | "pending_sent" | "pending_received" | "friends" | "loading";

interface Props {
  userId: number | string;
}

const FriendButton: React.FC<Props> = ({ userId }) => {
  const [status, setStatus] = useState<Status>("loading");
  const [busy, setBusy] = useState(false);
  const [, forceRender] = useState(0);

  useEffect(() => subscribe(() => forceRender((n) => n + 1)), []);

  const load = () => {
    axios
      .get(`/api/friends/status/${userId}`)
      .then((res) => setStatus(res.data?.status || "none"))
      .catch(() => setStatus("none"));
  };

  useEffect(load, [userId]);

  const act = (action: "request" | "accept" | "remove") => {
    if (busy) return;
    setBusy(true);
    axios
      .post(`/api/friends/${action}/${userId}`)
      .then((res) => setStatus(res.data?.status || "none"))
      .catch(() => {})
      .finally(() => setBusy(false));
  };

  if (status === "loading") return null;

  if (status === "friends") {
    return (
      <button type="button" className="btn btn-sm btn-outline-secondary" disabled={busy} onClick={() => act("remove")}>
        {t("Friends")} · {t("Remove Friend")}
      </button>
    );
  }

  if (status === "pending_sent") {
    return (
      <button type="button" className="btn btn-sm btn-outline-secondary" disabled={busy} onClick={() => act("remove")}>
        {t("Cancel Request")}
      </button>
    );
  }

  if (status === "pending_received") {
    return (
      <div className="d-flex gap-2">
        <button type="button" className="btn btn-sm btn-primary" disabled={busy} onClick={() => act("accept")}>
          {t("Accept")}
        </button>
        <button type="button" className="btn btn-sm btn-outline-secondary" disabled={busy} onClick={() => act("remove")}>
          {t("Decline")}
        </button>
      </div>
    );
  }

  return (
    <button type="button" className="btn btn-sm btn-primary" disabled={busy} onClick={() => act("request")}>
      {t("Add Friend")}
    </button>
  );
};

export default FriendButton;
