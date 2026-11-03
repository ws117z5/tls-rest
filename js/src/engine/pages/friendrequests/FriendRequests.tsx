import React from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import FriendRequestActions from "@engine/modules/friends/FriendRequestActions";
import { t } from "@engine/controllers/i18n";

interface ChatMember {
  id: number;
  userName: string;
  image?: string;
  status: string;
  message?: string;
}
interface ChatDto {
  id: number;
  type: string;
  myStatus: string;
  members: ChatMember[];
}

interface State {
  loading: boolean;
  chats: ChatDto[];
}

// Everyone who's sent the signed-in user a friend request they haven't
// responded to yet — one card per request, with its message and the same
// three response actions FriendButton shows on a profile page. See
// go/engine/pages/friendrequests (a pure menu entry — this reads/writes
// straight through go/engine/modules/chats' existing /api/chats* endpoints).
class FriendRequests extends PageComponent<{}, State> {
  protected isPage = true;
  protected requiresAuth = true;
  protected href = "friendrequests";
  protected title = "Friend requests";
  protected icon = "messages";

  constructor(props: {}) {
    super(props);
    this.state = { loading: true, chats: [] };
  }

  async componentDidMount() {
    await super.componentDidMount();
    this.load();
  }

  load = () => {
    this.setState({ loading: true });
    axios
      .get("/api/chats")
      .then((res) => {
        const all: ChatDto[] = res.data?.chats || [];
        const pending = all.filter((c) => c.type === "direct" && c.myStatus === "pending");
        this.setState({ chats: pending, loading: false });
      })
      .catch(() => this.setState({ loading: false }));
  };

  render() {
    const { loading, chats } = this.state;
    return (
      <div className="container py-4" style={{ maxWidth: 720 }}>
        <h1 className="h4 mb-3">{t("Friend requests")}</h1>
        {loading && <div className="text-muted">{t("Loading...")}</div>}
        {!loading && chats.length === 0 && <div className="text-muted">{t("No pending requests")}</div>}
        {chats.map((chat) => {
          const other = chat.members.find((m) => m.status !== "pending");
          const mine = chat.members.find((m) => m.status === "pending");
          if (!other) return null;
          return (
            <div key={chat.id} className="card mb-2">
              <div className="card-body">
                <div className="fw-semibold mb-1">{other.userName}</div>
                {mine?.message && <div className="text-muted small mb-2">&ldquo;{mine.message}&rdquo;</div>}
                <FriendRequestActions chatId={chat.id} userId={other.id} onResolved={this.load} />
              </div>
            </div>
          );
        })}
      </div>
    );
  }
}

export default FriendRequests;
