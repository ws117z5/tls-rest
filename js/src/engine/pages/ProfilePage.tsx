import React from "react";
import PageComponent from "@engine/PageComponent";
import FieldsetPage from "@engine/pages/FieldsetPage";

// Engine page: the current user's profile. A single, rights-filtered fieldset
// representation (no modes) backed by /api/profile.
class ProfilePage extends PageComponent {
    protected href = "profile";
    protected title = "Profile";
    protected requiresAuth = true;

    render() {
        return <FieldsetPage endpoint="/api/profile" moduleName="users" title="Profile" editable />;
    }
}

export default ProfilePage;