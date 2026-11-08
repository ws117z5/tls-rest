import React, { Component } from "react";
import axios from "axios";
import { t } from "@engine/controllers/i18n";

// Fires a one-time "I viewed this" ping (POST /api/views/{module}/{row},
// idempotent server-side per go/engine/modules/views) and displays the
// already-known count — count comes from the parent's own record (posts'
// views_count virtual field), not a separate GET, since the parent already
// has it by the time this mounts (see modules/posts/View.tsx).

interface Props {
  module: string;
  row: number | string | null | undefined;
  count: number;
}

class Views extends Component<Props> {
  componentDidMount() {
    const { module, row } = this.props;
    if (!module || row === undefined || row === null || row === "") return;
    axios.post(`/api/views/${module}/${row}`).catch(() => {});
  }

  render() {
    return (
      <span className="text-muted small">
        {this.props.count} {t("views")}
      </span>
    );
  }
}

export default Views;
