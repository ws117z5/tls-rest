import React, { useState } from "react";
import { Field } from "@engine/fields/FormLayout";
import Auth from "@controllers/auth";

// Params injected by ModulePage/WithLayout into custom containers.
interface CustomContainerProps {
  module: string;
  mode: string;
  record: any;
  values: Record<string, any>;
  fields: Record<string, any>;
  getValue: (name: string) => any;
  getField: (name: string) => any;
  getOptions: (name: string) => Record<string, any>;
  navigate: (to: string) => void;
  reload: () => void;
  submit: (form: any) => void;
  remove: (row: any) => void;
  modes: string[];
}

// Custom VIEW layout for the "posts" module. The parent chrome (ModulePage) owns
// the module name / Back / Edit; this only lays out the fields.
const PostsView: React.FC<CustomContainerProps> = (_props) => {
  const [metaOpen, setMetaOpen] = useState(false);

  return (
    <div className="posts-view">
      <div className="card shadow-sm my-3">

        {/* Foldable metadata — admin only. A thin bar with an arrow in the left
            corner, no title. Self-contained (no Bootstrap collapse JS). */}
        {Auth.isAdmin() && (
          <div className="border-bottom">
            <div
              role="button"
              onClick={() => setMetaOpen((o) => !o)}
              className="d-flex align-items-center bg-light px-3 py-2 text-muted"
              style={{ cursor: "pointer", userSelect: "none" }}
              title="Item details & metadata"
            >
              <span
                aria-hidden
                style={{
                  display: "inline-block",
                  fontSize: 11,
                  lineHeight: 1,
                  transition: "transform .15s ease",
                  transform: metaOpen ? "rotate(90deg)" : "rotate(0deg)",
                }}
              >
                ▶
              </span>
            </div>

            {metaOpen && (
              <div className="bg-light text-secondary small p-3 border-top">
                <div className="row g-2">
                  <div className="col-6 col-md-3"><Field name="id" /></div>
                  <div className="col-6 col-md-3"><Field name="uuid" /></div>
                  <div className="col-6 col-md-3"><Field name="created_by" /></div>
                  <div className="col-6 col-md-3"><Field name="access" /></div>
                  <div className="col-6 col-md-6"><Field name="created" /></div>
                  <div className="col-6 col-md-6"><Field name="updated" /></div>
                </div>
              </div>
            )}
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
};

export default PostsView;