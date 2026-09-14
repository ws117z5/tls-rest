import React, { Component } from "react";
import axios from "axios";
import Icon from "@engine/Icon";
import { t, subscribe } from "@engine/i18n";

// A like/dislike widget for one record, same polymorphic-target pattern as
// the comments thread (engine/modules/comments): talks to
//   GET  /api/likes/{module}/{row}  -> { likes, dislikes, mine }
//   POST /api/likes/{module}/{row}  -> { likes, dislikes, mine } (toggled)
// "mine" is the signed-in user's own reaction (1, -1, or 0); reacting the same
// way again removes it. Embedded by whichever module opts in (posts' custom
// view, each comment node, and the generic module page for "users").

interface Props {
  module: string;
  row: number | string | null | undefined;
  size?: "sm";
}

interface State {
  likes: number;
  dislikes: number;
  mine: number;
  busy: boolean;
}

class Likes extends Component<Props, State> {
  state: State = { likes: 0, dislikes: 0, mine: 0, busy: false };

  private unsubscribeI18n?: () => void;

  componentDidMount() {
    if (this.hasTarget()) this.load();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }

  componentWillUnmount() {
    this.unsubscribeI18n?.();
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
    try {
      const res = await axios.get(`/api/likes/${this.props.module}/${this.props.row}`);
      this.setState({
        likes: res.data?.likes || 0,
        dislikes: res.data?.dislikes || 0,
        mine: res.data?.mine || 0,
      });
    } catch {
      // Anonymous/unreadable — leave the counters at 0 rather than blocking render.
    }
  };

  private react = async (value: 1 | -1) => {
    if (this.state.busy || !this.hasTarget()) return;
    this.setState({ busy: true });
    try {
      const res = await axios.post(`/api/likes/${this.props.module}/${this.props.row}`, { value });
      this.setState({
        likes: res.data?.likes || 0,
        dislikes: res.data?.dislikes || 0,
        mine: res.data?.mine || 0,
        busy: false,
      });
    } catch {
      this.setState({ busy: false });
    }
  };

  render() {
    if (!this.hasTarget()) return null;
    const { likes, dislikes, mine, busy } = this.state;
    const sm = this.props.size === "sm";

    return (
      <div className={`likes-widget${sm ? " likes-widget-sm" : ""}`}>
        <button
          type="button"
          className={`btn btn-sm likes-btn${mine === 1 ? " likes-btn-active-up" : ""}`}
          disabled={busy}
          aria-pressed={mine === 1}
          title={t("Like")}
          onClick={() => this.react(1)}
        >
          <Icon name="like" />
          {likes}
        </button>
        <button
          type="button"
          className={`btn btn-sm likes-btn${mine === -1 ? " likes-btn-active-down" : ""}`}
          disabled={busy}
          aria-pressed={mine === -1}
          title={t("Dislike")}
          onClick={() => this.react(-1)}
        >
          <Icon name="dislike" />
          {dislikes}
        </button>
      </div>
    );
  }
}

export default Likes;
