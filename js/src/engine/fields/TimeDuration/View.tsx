import React, { Component } from "react";
import { formatDuration } from "./util";
interface Props { value?: number | string; format?: string; }
class TimeDurationView extends Component<Props> {
  render() {
    const total = typeof this.props.value === "string" ? parseInt(this.props.value, 10) || 0 : (this.props.value || 0);
    return <span>{formatDuration(total, this.props.format)}</span>;
  }
}
export default TimeDurationView;