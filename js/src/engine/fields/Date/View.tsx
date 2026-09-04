import React, { Component } from "react";
import { formatDate } from "@engine/controllers/dateformat";
import AppConfig from "@engine/controllers/Appconfig";

interface DateViewProps {
    value: string | number | Date;
}

// Renders a date/time using the user's configured date_format (global < group <
// user). Set date_format to include time tokens (e.g. "YYYY-MM-DD HH:mm") to show
// the time as well.
class DateView extends Component<DateViewProps> {
    render() {
        return <div>{formatDate(this.props.value, AppConfig.dateFormat())}</div>;
    }
}

export default DateView;