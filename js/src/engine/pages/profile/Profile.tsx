import PageComponent from "@engine/containers/PageComponent";
import Fieldset from "@engine/pages/Fieldset";

// Engine page: the current user's profile. A single, rights-filtered fieldset
// representation (no modes) backed by /api/profile.
class Profile extends PageComponent {
    protected href = "profile";
    protected title = "Profile";
    protected requiresAuth = true;

    render() {
        return <Fieldset endpoint="/api/profile" moduleName="users" title="Profile" editable />;
    }
}

export default Profile;