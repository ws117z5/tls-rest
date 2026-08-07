import React, { Component } from "react";

const locale = "US"

interface TimeViewProps {
    value: string | number | Date; // Accepts a date string, timestamp, or Date object
  }

class TimeView extends Component<TimeViewProps> {
    render() {
        let time = new Date(this.props.value);

        let timeStr: string;

        if(locale == "US") {
            timeStr = time.getMonth() + "/" + time.getDay() + "/" + time.getFullYear() + " " + time.getHours() + ":" + time.getMinutes()
        } else {
            timeStr = time.getDay() + "/" + time.getMonth() + "/" + time.getFullYear() + " " + time.getHours() + ":" + time.getMinutes()
        }

        return (
            <div>{timeStr}</div>
        )
    }
}

export default TimeView;