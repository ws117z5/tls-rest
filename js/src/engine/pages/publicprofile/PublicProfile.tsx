import React from "react";
import axios from "axios";
import PageComponent, { PageComponentProps } from "@engine/containers/PageComponent";
import Likes from "@engine/modules/likes/Likes";
import MessageThread from "@engine/modules/messages/MessageThread";
import FriendButton from "@engine/modules/friends/FriendButton";
import Auth from "@controllers/auth";
import { t, subscribe } from "@engine/i18n";

interface Profile {
  id: number;
  userName: string;
  image: string;
  created: string;
  self: boolean;
}

interface State {
  profile: Profile | null;
  loading: boolean;
  error: string;
}

function whenText(v: string): string {
  if (!v) return "";
  const d = new Date(v);
  return isNaN(d.getTime()) ? String(v) : d.toLocaleString();
}

// The public, privacy-safe view of another user's account: username, avatar
// and join date only — never first/last name or email (those stay behind the
// admin-only "users" module and the signed-in user's own /profile page).
// Reached only via engine/pages/profile/Profile.tsx's extraRoutes ("/u/:id")
// — that page is a real, backend-registered menu entry (a stable, unique
// href), which is what makes extraRoutes actually get processed by app.tsx;
// this component itself is never independently routed or shown in the menu
// (href/isPage left at PageComponent's defaults, same as Room.tsx under
// pages/papers).
class PublicProfile extends PageComponent<PageComponentProps, State> {
  state: State = { profile: null, loading: true, error: "" };
  private unsubscribeI18n?: () => void;

  async componentDidMount() {
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    await this.load();
  }

  componentDidUpdate(prev: PageComponentProps) {
    if (prev.params?.id !== this.props.params?.id) this.load();
  }

  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  private load = async () => {
    const id = this.props.params?.id;
    if (!id) return;
    this.setState({ loading: true, error: "" });
    try {
      const res = await axios.get(`/api/users/${id}/public`);
      this.setState({ profile: res.data, loading: false });
    } catch (e: any) {
      const msg =
        e?.response?.status === 404 ? t("Profile not found.") : t("Sign in to view profiles.");
      this.setState({ profile: null, loading: false, error: msg });
    }
  };

  render() {
    if (!Auth.isAuthenticated()) {
      return (
        <div className="container py-4" style={{ maxWidth: 640 }}>
          <p className="text-muted">{t("Sign in to view profiles.")}</p>
        </div>
      );
    }

    const { profile, loading, error } = this.state;

    if (loading) {
      return (
        <div className="d-flex justify-content-center p-4">
          <div className="spinner-border" role="status" />
        </div>
      );
    }

    if (error || !profile) {
      return (
        <div className="container py-4" style={{ maxWidth: 640 }}>
          <div className="alert alert-warning">{error || t("Profile not found.")}</div>
        </div>
      );
    }

    return (
      <div className="container py-4" style={{ maxWidth: 640 }}>
        <div className="card">
          <div className="card-body">
            <div className="d-flex align-items-center gap-3 mb-3">
              {profile.image ? (
                <img
                  src={profile.image}
                  alt=""
                  style={{ width: 72, height: 72, borderRadius: "50%", objectFit: "cover" }}
                />
              ) : (
                <div
                  className="rounded-circle bg-light"
                  style={{ width: 72, height: 72 }}
                  aria-hidden="true"
                />
              )}
              <div>
                <h1 className="h4 mb-1">{profile.userName}</h1>
                <div className="text-muted small">
                  {t("Member since")} {whenText(profile.created)}
                </div>
              </div>
            </div>

            <div className="d-flex align-items-center gap-3">
              <Likes module="users" row={profile.id} />
              {!profile.self && <FriendButton userId={profile.id} />}
            </div>
          </div>
        </div>

        {!profile.self && (
          <div className="card mt-3">
            <div className="card-body">
              <h2 className="h6 text-uppercase text-muted mb-3">
                {t("Message")} {profile.userName}
              </h2>
              <MessageThread otherUserId={profile.id} />
            </div>
          </div>
        )}
      </div>
    );
  }
}

export default PublicProfile;
