import React from "react";
import { Field } from "@engine/fields/FormLayout";

// Custom edit layout for the "posts" module. Position fields freely; each
// <Field name="..."/> is wired to the surrounding fieldset form automatically.
const PostsEdit: React.FC = () => (
  <div className="posts-edit">
    <div className="row">
      <div className="col-md-8">
        <Field name="created_by" />
        <Field name="created" />
        <Field name="uuid" />
        <Field name="title" />
        <Field name="content" />
      </div>
      <div className="col-md-4">
        <Field name="images" />
        <Field name="public" />
      </div>
    </div>
  </div>
);

export default PostsEdit;