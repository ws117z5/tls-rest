import React, { Component } from "react";
import "./fieldsmenu.css";
import {Motion, spring} from 'react-motion';

class FieldsMenu extends Component {
    state = {
        xPos: "0px",
        yPos: "0px",
        visible: false,
        fields: {}
    }

    handleContextMenu = (e) => {
        e.preventDefault();

        //console.log("right click");
        this.setState({
            xPos: `${e.pageX}px`,
            yPos: `${e.pageY-50}px`,
            visible: true,
          });
    }

    handleClick = (e) => {
        //console.log("left click");
        let element = document.querySelector(".contextMenu");
        if (e.target !== element && !element.contains(e.target)) {
            this.setState({
                visible: false,
              });
          }
        if (e.target.type === "checkbox" && this.props.fields.hasOwnProperty(e.target.value)) {
            let el = e.target.value
            this.props.fields[el] = !this.props.fields[el]
            this.setState({
                fields: this.props.fields,
              });

            this.props.onFieldsetChange(this.props.fields);
        }
    }

    componentDidMount() {
        document.addEventListener("click", this.handleClick);
        document.querySelector(".tableHead")?.addEventListener("contextmenu", this.handleContextMenu);
    }

    componentWillUnmount() {
        document.removeEventListener("click", this.handleClick);
        document.querySelector(".tableHead")?.removeEventListener("contextmenu", this.handleContextMenu);
    }

    componentDidUpdate(prevProps) {
        if (this.props.fields !== prevProps.fields) {
            this.setState({ fields: this.props.fields });
        }
    }
    
    render() {
        const { visible, xPos, yPos } = this.state;

        return (
            <Motion defaultStyle={{ opacity: 0 }} style={{ opacity: !visible ? spring(0) : spring(1) }} >
            {(interpolatedStyle) => (
                <>
                    <div className="contextMenu" style={{
                            //visible: visible,
                            top: yPos,
                            left: xPos,
                            opacity: interpolatedStyle.opacity,
                            zIndex: visible ? 999 : -1
                        }}>
                            <p>Fieldset</p>
                        <ul className="menu">
                            { 
                                Object.keys(this.props.fields)?.map((el) => {
                                    return (
                                        <li key={el}>
                                            <label>
                                                <input type="checkbox" checked={this.props.fields[el]} value={el} onChange={() => {}}></input>
                                                {el}
                                            </label>
                                        </li>
                                    )
                                })
                            }
                        </ul>
                    </div>
            </>
            )}
            </Motion>
        )
    }
}

export default FieldsMenu;