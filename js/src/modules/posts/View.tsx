import React from "react";
import { Field } from "@engine/fields/FormLayout";
import Auth from "@controllers/auth";

// Params injected by ModulePage/WithLayout into custom containers. Use these to
// check field data values and field options, e.g.:
//
//   if (getValue("public")) { ... }
//   const opts = getOptions("images");        // that field's options
//   const isTable = getField("rows")?.type === "Table";
interface CustomContainerProps {
  module: string;
  mode: string;                                 // "view" | "edit" | "create"
  record: any;                                  // the loaded record
  values: Record<string, any>;                  // live form values
  fields: Record<string, any>;                  // field metas (type, options, ...)
  getValue: (name: string) => any;
  getField: (name: string) => any;
  getOptions: (name: string) => Record<string, any>;
  navigate: (to: string) => void;
  reload: () => void;
  submit: (form: any) => void;
  remove: (row: any) => void;
  modes: string[];
}

// Custom VIEW layout for the "posts" module. The parent container (ModulePage)
// owns the standard chrome — module name, mode, and Save/Back actions — so this
// component only lays out the fields. Each <Field name="..."/> is wired to the
// surrounding fieldset form automatically.
const PostsView: React.FC<CustomContainerProps> = (_props) => (
  <div className="posts-view">
    <div className="card shadow-sm my-3">

      {/* Foldable metadata header — admin only */}
      {Auth.isAdmin() && (
        <div className="accordion" id="metadataAccordion">
          <div className="accordion-item border-0 border-bottom">
            <h2 className="accordion-header" id="headingMetadata">
              <button
                className="accordion-button collapsed bg-light py-2 px-3 text-muted small"
                type="button"
                data-bs-toggle="collapse"
                data-bs-target="#collapseMetadata"
                aria-expanded="false"
                aria-controls="collapseMetadata">
                <i className="bi bi-info-circle me-2"></i> View Item Details &amp; Metadata
              </button>
            </h2>

            <div
              id="collapseMetadata"
              className="accordion-collapse collapse"
              aria-labelledby="headingMetadata"
              data-bs-parent="#metadataAccordion">
              <div className="accordion-body bg-light text-secondary small border-top">
                <div className="row g-2">
                  <div className="col-6 col-md-3"><Field name="id" /></div>
                  <div className="col-6 col-md-3"><Field name="uuid" /></div>
                  <div className="col-6 col-md-3"><Field name="created_by" /></div>
                  <div className="col-6 col-md-3"><Field name="access" /></div>
                  <div className="col-6 col-md-6"><Field name="created" /></div>
                  <div className="col-6 col-md-6"><Field name="updated" /></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main content body */}
      <div className="card-body">
        <div className="mb-3 border-bottom pb-2">
          <h3 className="card-title h4 mb-1"><Field name="title" /></h3>
          <p className="card-subtitle text-muted fs-6">By <Field name="author" /></p>
        </div>

        <div className="card-text text-dark">
          <Field name="content" />
        </div>
      </div>

    </div>
  </div>
);

export default PostsView;