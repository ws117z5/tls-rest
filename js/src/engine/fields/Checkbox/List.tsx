import React, { Component } from "react";

// List-mode checkbox cell: read-only display of a boolean value. Checkboxes are
// never interactive in list mode (the only interactive checkbox in a list row is
// the row-selection box rendered by FieldsetList).
class CheckboxList extends Component<BooleanProps> {
    render() {
        return (
            <input
                type="checkbox"
                checked={!!this.props.value}
                readOnly
                disabled
            />
        );
    }
}

export default CheckboxList;