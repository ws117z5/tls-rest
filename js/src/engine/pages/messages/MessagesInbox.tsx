import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import ConversationList from "@engine/modules/messages/ConversationList";
import { t } from "@engine/controllers/i18n";

// Menu-visible inbox: every conversation the signed-in user is part of. The
// same list (ConversationList) is also embedded directly in the user's own
// profile page (engine/pages/profile/Profile.tsx).
class MessagesInbox extends PageComponent {
  protected isPage = true;
  protected requiresAuth = true;
  protected href = "messages";
  protected title = "Messages";
  protected icon = "messages";

  render() {
    return (
      <div className="container py-4" style={{ maxWidth: 720 }}>
        <h1 className="h4 mb-3">{t("Messages")}</h1>
        <ConversationList />
      </div>
    );
  }
}

export default MessagesInbox;
