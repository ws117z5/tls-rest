import React, { useState } from "react";
import { Field } from "@engine/fields/FormLayout";
import CommentsThread from "@engine/modules/comments/CommentsThread";
import Auth from "@controllers/auth";
import useT from "@engine/useT";

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

// Fields shown in the foldable admin metadata panel, in display order.
const META_FIELDS = ["id", "uuid", "created_by", "access", "created", "updated"];

// Custom VIEW layout for the "posts" module. The parent chrome (ModulePage) owns
// the module name / Back / Edit; this only lays out the fields.
const PostsView: React.FC<CustomContainerProps> = (props) => {
  const t = useT();
  const [metaOpen, setMetaOpen] = useState(false);

  const title = props.getValue("title");
  const author = props.getValue("author");
  const postId = props.record?.id ?? props.getValue("id");

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
              title={t("Item details & metadata")}
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
                <table className="table table-sm mb-0 align-middle">
                  <tbody>
                    {META_FIELDS.map((name) => {
                      const f = props.getField(name);
                      if (!f) return null;
                      return (
                        <tr key={name}>
                          <th
                            className="fw-semibold text-nowrap"
                            style={{ width: "30%", whiteSpace: "nowrap" }}
                          >
                            {f.label ? t(f.label) : name}
                            {f.description && (
                              <div className="text-muted fw-normal">
                                {t(f.description)}
                              </div>
                            )}
                          </th>
                          <td>
                            <Field name={name} label="" />
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* Main content body */}
        <div className="card-body">
          <header className="posts-hero mb-4 pb-3 border-bottom">
            <h1
              className="mb-2"
              style={{
                fontFamily: "Georgia, 'Times New Roman', serif",
                fontSize: "2.75rem",
                fontWeight: 700,
                lineHeight: 1.15,
                letterSpacing: "-0.015em",
              }}
            >
              {title || <span className="text-muted">{t("Untitled")}</span>}
            </h1>
            <div
              style={{
                fontSize: "1.15rem",
                fontStyle: "italic",
                color: "#6c757d",
              }}
            >
              {t("by")}{" "}
              <span style={{ color: "#343a40", fontWeight: 600 }}>
                {author || t("Unknown")}
              </span>
            </div>
            <div
              aria-hidden
              style={{
                height: 3,
                width: 64,
                marginTop: "1rem",
                borderRadius: 2,
                background: "linear-gradient(90deg, #0d6efd, #6610f2)",
              }}
            />
          </header>

          <div className="card-text text-dark">
            <Field name="content" />
          </div>
        </div>

        {/* Threaded comments (engine/modules/comments), loaded from the
            /api/comments REST API for this post. */}
        <div className="card-footer bg-white border-top">
          <CommentsThread module="posts" row={postId} />
        </div>

      </div>
    </div>
  );
};

export default PostsView;
