import React, { Component, ChangeEvent } from "react";
import ReactDOM from "react-dom";
import Input from "../presentational/Input";

interface FormContainerProps {
  seo_title?: string;
}

// 1. Define the state interface
interface FormContainerState {
  seo_title: string;
  [key: string]: string; // Allows dynamic keys like [event.target.id]
}

class FormContainer extends Component<FormContainerProps, FormContainerState> {
  constructor(props) {
    super(props);
    this.state = {
      seo_title: props.seo_title || ""
    };
    this.handleChange = this.handleChange.bind(this);
  }

  handleChange(event: ChangeEvent<HTMLInputElement>) {
    const { id, value } = event.target;
    this.setState({ [id]: value } as Pick<FormContainerState, keyof FormContainerState>);
  }

  render() {
    const { seo_title } = this.state;
    return (
      <form id="article-form">
        <Input
          text="SEO title"
          label="seo_title"
          type="text"
          id="seo_title"
          value={seo_title}
          handleChange={this.handleChange}
        />
      </form>
    );
  }
}
export default FormContainer;

