import React from "react";
import PageComponent from "@engine/PageComponent";
import { FieldsetProvider, FieldsetList, MODES } from "@engine/fields";

class UsersPage extends PageComponent {
  protected isPage = true;
  protected requiresAuth = true;
  protected title = "Users";
  protected href = "users";

  render() {
    // Row data is loaded by PageComponent.fetchDefaultApiData (GET /users) into
    // this.state.Data; FieldsetProvider supplies the column definitions from
    // /api/modules/users/fieldset, and FieldsetList renders them together.
    return (
      <div className="base">
        This is users
        <FieldsetProvider module="users" mode={MODES.LIST}>
          <FieldsetList
            data={this.state.Data}
            sortable={true}
            showActions={true}
          />
        </FieldsetProvider>
      </div>
    );
  }
}

export default UsersPage;