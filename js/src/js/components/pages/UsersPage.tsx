import React from "react";
import UsersList from "@containers/UserList";
import PageComponent from "@controllers/PageComponent";
class UsersPage extends PageComponent {
  protected isPage = true;
  protected requiresAuth = true;
  protected title = "Users";
  protected href = "users";

  render() {
    const fieldsSelected: { [key: string]: boolean } = {};

    Object.keys(this.state.Fieldset).forEach((el) => {
      fieldsSelected[el] = true;
    });
    
    return (
      <div className="base">
        This is users
        <UsersList
          data={this.state.Data}
          fieldset={this.state.Fieldset}
          fieldsSelected={fieldsSelected}
        />
      </div>
    );
  }
}


export default UsersPage;