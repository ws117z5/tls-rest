import React, { useCallback, useEffect, useState } from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import Fieldset from "@engine/pages/Fieldset";
import ConversationList from "@engine/modules/messages/ConversationList";
import PublicProfile from "@engine/pages/publicprofile/PublicProfile";
import Auth from "@controllers/auth";
import { t } from "@engine/controllers/i18n";

// Google Calendar connect/disconnect — requesting access lives here, not on
// the Calendar page. Once connected, each event gets its own "Sync with
// Google Calendar" checkbox (default from the "New events sync..." field in
// the fieldset above) that pushes it to this account on save — this app
// never pulls Google's events in, only ever pushes its own out.
const GoogleCalendarCard: React.FC = () => {
    const [connected, setConnected] = useState<boolean | null>(null);
    const [notice, setNotice] = useState<string>("");

    const refreshStatus = useCallback(() => {
        axios
            .get("/api/profile/google/status")
            .then((res) => setConnected(!!(res.data && res.data.connected)))
            .catch(() => setConnected(false));
    }, []);

    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        if (params.get("connected")) setNotice(t("Google Calendar connected."));
        if (params.get("error")) setNotice(t("Couldn't connect Google Calendar — please try again."));
        if (params.get("connected") || params.get("error")) {
            window.history.replaceState({}, "", window.location.pathname);
        }
        refreshStatus();
    }, [refreshStatus]);

    const disconnect = async () => {
        await axios.post("/api/profile/google/disconnect");
        refreshStatus();
    };

    return (
        <div className="card module-page-card mt-4">
            <div className="card-body">
                <h5 className="card-title h6 text-uppercase text-muted mb-3">{t("Google Calendar")}</h5>
                {connected === null ? (
                    <span className="text-muted small">{t("Loading…")}</span>
                ) : connected ? (
                    <div className="d-flex align-items-center gap-2 flex-wrap">
                        <span className="text-muted small">{t("Connected")}</span>
                        <button className="btn btn-sm btn-outline-secondary" onClick={disconnect}>
                            {t("Disconnect")}
                        </button>
                    </div>
                ) : (
                    <a className="btn btn-sm btn-outline-primary" href="/api/profile/google/connect">
                        {t("Connect Google Calendar")}
                    </a>
                )}
                {notice && <div className="text-muted small mt-2">{notice}</div>}
            </div>
        </div>
    );
};

// Generates a bearer token for the local Claude Code "Stop" hook (any
// session, any project on this machine — go/modules/claudenotify) to post
// "task done" notifications with. Shown once, like any other bearer token
// this app issues — there's no way to recover it later, only mint a new one.
const ClaudeNotifyTokenCard: React.FC = () => {
    const [token, setToken] = useState<string>("");
    const [error, setError] = useState<string>("");

    const generate = async () => {
        setError("");
        try {
            const res = await axios.post("/api/profile/claude-token");
            setToken(res.data.token || "");
        } catch {
            setError(t("Couldn't generate a token — please try again."));
        }
    };

    return (
        <div className="card module-page-card mt-4">
            <div className="card-body">
                <h5 className="card-title h6 text-uppercase text-muted mb-3">{t("Claude notifications")}</h5>
                <p className="text-muted small mb-3">
                    {t("Paste this into the Claude Code \"Stop\" hook on your machine so it can post \"task done\" notifications here.")}
                </p>
                {token ? (
                    <textarea className="form-control font-monospace small" rows={2} readOnly value={token} onFocus={(e) => e.target.select()} />
                ) : (
                    <button className="btn btn-sm btn-outline-primary" onClick={generate}>
                        {t("Generate token")}
                    </button>
                )}
                {error && <div className="text-danger small mt-2">{error}</div>}
            </div>
        </div>
    );
};

// Engine page: the current user's profile. A single, rights-filtered fieldset
// representation (no modes) backed by /api/profile, plus the user's message
// inbox.
//
// Also hosts the "/u/:id" public-profile route via extraRoutes: app.tsx only
// reads a page's extraRoutes when that page is itself a real, backend-known
// menu entry with a stable href (this one, "profile") — see PublicProfile.tsx
// for why hosting it there directly (href left at the PageComponent default)
// doesn't work.
class Profile extends PageComponent {
    protected href = "profile";
    protected title = "Profile";
    protected requiresAuth = true;

    static extraRoutes = [{ href: "/u/:id", component: PublicProfile }];

    render() {
        return (
            <div className="container-fluid module-page pt-4">
                <div className="module-page-header d-flex justify-content-between align-items-center mb-3">
                    <h1 className="h4 mb-0">{t("Profile")}</h1>
                    <button className="btn btn-secondary" onClick={() => Auth.logout()}>
                        {t("Logout")}
                    </button>
                </div>
                <Fieldset endpoint="/api/profile" moduleName="users" editable />
                <GoogleCalendarCard />
                <ClaudeNotifyTokenCard />
                <div className="card module-page-card mt-4">
                    <div className="card-body">
                        <h5 className="card-title h6 text-uppercase text-muted mb-3">{t("Messages")}</h5>
                        <ConversationList />
                    </div>
                </div>
            </div>
        );
    }
}

export default Profile;