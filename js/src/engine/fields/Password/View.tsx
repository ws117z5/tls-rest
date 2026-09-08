import React, { Component } from "react";
interface Props { value?: string; }
// Never reveal a stored password; show a fixed mask if a value is set.
class PasswordView extends Component<Props> {
  render() {
    return <span>{this.props.value ? "••••••••" : ""}</span>;
  }
}
export default PasswordView;