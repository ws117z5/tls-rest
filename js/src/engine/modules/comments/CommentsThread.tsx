import React, { Component } from "react";
import axios from "axios";
import MarkdownRender from "@engine/fields/Markdown/controllers/MarkdownRender";

// A threaded discussion for one record. Talks to the polymorphic comments API:
//   GET  /api/comments/{module}/{row}  -> { comments: tree }
//   POST /api/comments/{module}/{row}  -> { id }
// A reply is just a comment whose target is ("comments", parentCommentId), so
// the thread nests arbitrarily deep. New comments are Markdown; images are not
// offered here.

export interface CommentNode {
  id: number;
  author: string;
  body: string;
  created: string;
  replies?: CommentNode[];
}

interface Props {
  module: string;
  row: number | string | null | undefined;
}

interface State {
  tree: CommentNode[];
  loading: boolean;
  error: string;
  activeReply: number | null;
  submitting: boolean;
}

function whenText(v: string): string {
  if (!v) return "";
  const d = new Date(v);
  return isNaN(d.getTime()) ? String(v) : d.toLocaleString();
}

function countAll(nodes: CommentNode[]): number {
  return nodes.reduce(
    (sum, n) => sum + 1 + (n.replies ? countAll(n.replies) : 0),
    0
  );
}

// --- Composer -------------------------------------------------------------

interface ComposerProps {
  onSubmit: (body: string) => Promise<boolean | void> | boolean | void;
  submitting?: boolean;
  placeholder?: string;
  submitLabel?: string;
}
interface ComposerState {
  body: string;
  preview: boolean;
}

class Composer extends Component<ComposerProps, ComposerState> {
  state: ComposerState = { body: "", preview: false };

  private send = async () => {
    const ok = await this.props.onSubmit(this.state.body);
    if (ok !== false) this.setState({ body: "", preview: false });
  };

  render() {
    const { submitting, placeholder, submitLabel = "Comment" } = this.props;
    const { body, preview } = this.state;
    const empty = !body.trim();

    return (
      <div className="comment-composer border rounded p-2 bg-light">
        <div className="btn-group btn-group-sm mb-1" role="group">
          <button
            type="button"
            className={`btn btn-outline-secondary ${preview ? "" : "active"}`}
            onClick={() => this.setState({ preview: false })}
          >
            Write
          </button>
          <button
            type="button"
            className={`btn btn-outline-secondary ${preview ? "active" : ""}`}
            onClick={() => this.setState({ preview: true })}
            disabled={empty}
          >
            Preview
          </button>
        </div>

        {preview ? (
          <div
            className="form-control bg-white"
            style={{ minHeight: 80, overflow: "auto" }}
          >
            <MarkdownRender value={body} />
          </div>
        ) : (
          <textarea
            className="form-control"
            rows={3}
            value={body}
            placeholder={placeholder}
            onChange={(e) => this.setState({ body: e.target.value })}
          />
        )}

        <div className="d-flex justify-content-end mt-2">
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={empty || submitting}
            onClick={this.send}
          >
            {submitting ? "Saving…" : submitLabel}
          </button>
        </div>
      </div>
    );
  }
}

// --- Thread --------------------------------------------------------------

class CommentsThread extends Component<Props, State> {
  state: State = {
    tree: [],
    loading: false,
    error: "",
    activeReply: null,
    submitting: false,
  };

  componentDidMount() {
    if (this.hasTarget()) this.load();
  }

  componentDidUpdate(prev: Props) {
    if (prev.row !== this.props.row || prev.module !== this.props.module) {
      if (this.hasTarget()) this.load();
    }
  }

  private hasTarget(): boolean {
    const r = this.props.row;
    return !!this.props.module && r !== undefined && r !== null && r !== "";
  }

  private load = async () => {
    this.setState({ loading: true, error: "" });
    try {
      const res = await axios.get(
        `/api/comments/${this.props.module}/${this.props.row}`
      );
      const tree = (res.data && res.data.comments) || [];
      this.setState({
        tree: Array.isArray(tree) ? tree : [],
        loading: false,
      });
    } catch {
      this.setState({ error: "Could not load comments.", loading: false });
    }
  };

  private submit = async (
    targetModule: string,
    targetRow: number | string,
    body: string
  ): Promise<boolean> => {
    const text = body.trim();
    if (!text || this.state.submitting) return false;
    this.setState({ submitting: true });
    try {
      await axios.post(`/api/comments/${targetModule}/${targetRow}`, {
        body: text,
      });
      this.setState({ submitting: false, activeReply: null });
      await this.load();
      return true;
    } catch {
      this.setState({ submitting: false, error: "Could not post comment." });
      return false;
    }
  };

  private renderNode = (n: CommentNode, depth: number): React.ReactNode => {
    const replying = this.state.activeReply === n.id;
    return (
      <li key={n.id} className="comment-node" style={{ marginTop: "1rem" }}>
        <div className="mb-1">
          <span className="fw-semibold">{n.author || "Anonymous"}</span>
          <span className="text-muted small ms-2">{whenText(n.created)}</span>
        </div>
        <div className="comment-body">
          <MarkdownRender value={n.body} />
        </div>
        <button
          type="button"
          className="btn btn-link btn-sm p-0 text-decoration-none"
          onClick={() =>
            this.setState({ activeReply: replying ? null : n.id })
          }
        >
          {replying ? "Cancel" : "Reply"}
        </button>

        {replying && (
          <div className="mt-2">
            <Composer
              submitting={this.state.submitting}
              placeholder={`Reply to ${n.author || "comment"}…`}
              submitLabel="Reply"
              onSubmit={(body) => this.submit("comments", n.id, body)}
            />
          </div>
        )}

        {n.replies && n.replies.length > 0 && (
          <ul
            className="list-unstyled"
            style={{
              marginLeft: depth < 4 ? "1.5rem" : "0.75rem",
              borderLeft: "2px solid #e9ecef",
              paddingLeft: "1rem",
            }}
          >
            {n.replies.map((c) => this.renderNode(c, depth + 1))}
          </ul>
        )}
      </li>
    );
  };

  render() {
    if (!this.hasTarget()) return null;
    const { tree, loading, error } = this.state;
    const count = countAll(tree);

    return (
      <section className="comments-thread">
        <h5 className="h6 text-uppercase text-muted mb-3">
          {count > 0 ? `Comments (${count})` : "Comments"}
        </h5>

        {error && <div className="alert alert-warning py-2">{error}</div>}

        <Composer
          submitting={this.state.submitting}
          placeholder="Add a comment… (Markdown supported)"
          submitLabel="Post comment"
          onSubmit={(body) =>
            this.submit(this.props.module, this.props.row as string | number, body)
          }
        />

        {loading && tree.length === 0 ? (
          <p className="text-muted small mt-3">Loading…</p>
        ) : tree.length === 0 ? (
          <p className="text-muted small mt-3">No comments yet. Be the first.</p>
        ) : (
          <ul className="list-unstyled mt-3">
            {tree.map((n) => this.renderNode(n, 0))}
          </ul>
        )}
      </section>
    );
  }
}

export default CommentsThread;
