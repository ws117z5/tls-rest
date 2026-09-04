import React, { Component } from "react";
import { formatDate } from "@engine/controllers/dateformat";
import AppConfig from "@engine/controllers/Appconfig";

interface DateListProps {
    value: string | number | Date;
}

class DateList extends Component<DateListProps> {
    render() {
        return <span>{formatDate(this.props.value, AppConfig.dateFormat())}</span>;
    }
}

export default DateList;