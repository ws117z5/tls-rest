import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import { FieldsetProvider, FieldsetList, MODES } from "@engine/fields";
import { t, subscribe } from "@engine/i18n";

class UsersPage extends PageComponent {
  protected isPage = true;
  protected requiresAuth = true;
  protected title = "Users";
  protected href = "users";

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  render() {
    // Row data is loaded by PageComponent.fetchDefaultApiData (GET /users) into
    // this.state.Data; FieldsetProvider supplies the column definitions from
    // /api/modules/users/fieldset, and FieldsetList renders them together.
    return (
      <div className="base">
        {t("This is users")}
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