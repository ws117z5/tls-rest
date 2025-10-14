import React from "react";
import UsersList from "../container/UserList";
import Request from "../../controllers/request";
import PageComponent from "../../controllers/PageComponent";

interface UsersPageState {
  Data: any[];
  Fieldset: { [key: string]: any };
}

class UsersPage extends PageComponent<{}, UsersPageState> {
  static href = "users";
  static title = "Users";

  constructor(props: {}) {
    super(props);
    this.state = { Data: [], Fieldset: {} };
  }

  componentDidMount() {
    // axios.get(`https://localhost:8080/users/${this.props.subreddit}`)
    Request.apiListRequest("users", this);
  }

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