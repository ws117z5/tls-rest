import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import Fieldset from "@engine/pages/Fieldset";
import ConversationList from "@engine/modules/messages/ConversationList";
import PublicProfile from "@engine/pages/publicprofile/PublicProfile";
import { t } from "@engine/i18n";

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
            <>
                <Fieldset endpoint="/api/profile" moduleName="users" title="Profile" editable />
                <div className="container" style={{ maxWidth: 720 }}>
                    <h2 className="h5 mt-4 mb-3">{t("Messages")}</h2>
                    <ConversationList />
                </div>
            </>
        );
    }
}

export default Profile;