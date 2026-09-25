import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import Fieldset from "@engine/pages/Fieldset";
import ConversationList from "@engine/modules/messages/ConversationList";
import PublicProfile from "@engine/pages/publicprofile/PublicProfile";
import Auth from "@controllers/auth";
import { t } from "@engine/controllers/i18n";

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