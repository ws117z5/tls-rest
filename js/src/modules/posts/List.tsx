import React from "react";
import { ModuleViewProps } from "@engine/controllers/registry";

// Strip markdown/HTML to a short plain-text excerpt of the body.
function excerpt(body: string, max = 180): string {
  if (!body) return "";
  const text = body
    .replace(/<[^>]*>/g, " ")          // html tags
    .replace(/[#>*_`~\-]{1,}/g, " ")   // common markdown marks
    .replace(/!?\[[^\]]*\]\([^)]*\)/g, " ") // links/images
    .replace(/\s+/g, " ")
    .trim();
  return text.length > max ? text.slice(0, max).trimEnd() + "…" : text;
}

// Custom LIST view for the "posts" module: title, author and a body excerpt per
// post. Clicking a card opens its view.
const PostsList: React.FC<ModuleViewProps> = ({ data, navigate, module }) => {
  const rows = Array.isArray(data) ? data : [];
  if (rows.length === 0) {
    return <div className="text-muted p-3">No posts yet.</div>;
  }
  return (
    <div className="d-flex flex-column gap-2">
      {rows.map((row: any) => (
        <div
          key={row.id}
          className="card"
          style={{ cursor: "pointer" }}
          onClick={() => navigate(`/${module}/${row.id}`)}
        >
          <div className="card-body py-2">
            <h5 className="card-title mb-1">{row.title || `#${row.id}`}</h5>
            <div className="text-muted small mb-2">By {row.author || "—"}</div>
            <div className="text-secondary small mb-0">{excerpt(row.content)}</div>
          </div>
        </div>
      ))}
    </div>
  );
};

export default PostsList;