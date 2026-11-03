import React, { useState } from "react";
import axios from "axios";
import { t } from "@engine/controllers/i18n";

// The three ways to respond to an incoming friend request — shared by
// FriendButton (on a public profile) and the FriendRequests inbox page. See
// go/engine/modules/chats' handleAccept/handleBlock/handleReportUser.
interface Props {
  chatId: number | string;
  userId: number | string;
  onResolved: () => void;
}

const FriendRequestActions: React.FC<Props> = ({ chatId, userId, onResolved }) => {
  const [busy, setBusy] = useState(false);

  const act = (promise: Promise<any>) => {
    if (busy) return;
    setBusy(true);
    promise.then(onResolved).catch(() => {}).finally(() => setBusy(false));
  };

  return (
    <div className="d-flex gap-2">
      <button
        type="button"
        className="btn btn-sm btn-primary"
        disabled={busy}
        onClick={() => act(axios.post(`/api/chats/${chatId}/accept`))}
      >
        {t("Add friend")}
      </button>
      <button
        type="button"
        className="btn btn-sm btn-outline-secondary"
        disabled={busy}
        onClick={() => act(axios.post(`/api/blocks/${userId}`))}
      >
        {t("Block")}
      </button>
      <button
        type="button"
        className="btn btn-sm btn-outline-danger"
        disabled={busy}
        onClick={() => act(axios.post(`/api/users/${userId}/report`))}
      >
        {t("Block and report")}
      </button>
    </div>
  );
};

export default FriendRequestActions;
