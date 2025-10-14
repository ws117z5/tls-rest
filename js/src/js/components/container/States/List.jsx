import React, { Component } from "react";
import { TimeList, TextList, CheckboxList } from "../Fields";

import FieldsMenu from "./FieldsMenu";
import "./list.css";

//todo
const components = {
    "time.Time": TimeList,
    "int64": TextList
};

class StateList extends Component {
    state = {
        data: [],
        fieldset: {},
        fieldsSelected: {}
    };

    componentDidUpdate(prevProps) {
    if (this.props.data !== prevProps.data) {
        this.setState({
            data: this.props.data,
            fieldset: this.props.fieldset,
            fieldsSelected: this.props.fieldsSelected
        });
    }
}

    initResizeEvents = () => {
        // Our real web app uses Vue.js but I'll stick to plain JavaScript here

        const min = 150;
        // The max (fr) values for grid-template-columns
        const columnTypeToRatioMap = {
            numeric: 1,
            'text-short': 1.67,
            'text-long': 3.33,
        };

        const table = document.querySelector('table');
        /*
        The following will soon be filled with column objects containing
        the header element and their size value for grid-template-columns 
        */
        const columns = [];
        let headerBeingResized;

        // The next three functions are mouse event callbacks

        // Where the magic happens. I.e. when they're actually resizing
        const onMouseMove = (e) => requestAnimationFrame(() => {
            console.log('onMouseMove');
            
            // Calculate the desired width
            var horizontalScrollOffset = document.documentElement.scrollLeft;
            const width = (horizontalScrollOffset + e.clientX) - headerBeingResized.offsetLeft;
            
            // Update the column object with the new size value
            const column = columns.find(({ header }) => header === headerBeingResized);
            column.size = Math.max(min, width) + 'px'; // Enforce our minimum
            
            // For the other headers which don't have a set width, fix it to their computed width
            columns.forEach((column) => {
                if(column.size.startsWith('minmax')){ // isn't fixed yet (it would be a pixel value otherwise)
                    column.size = parseInt(column.header.clientWidth, 10) + 'px';
                }
            });
            
            /* 
                Update the column sizes
                Reminder: grid-template-columns sets the width for all columns in one value
            */
            table.style.gridTemplateColumns = columns.map(
                ({ header, size }) => size).join(' ');
        });

        //onMouseMove();

        // Clean up event listeners, classes, etc.
        const onMouseUp = () => {
            console.log('onMouseUp');
            
            window.removeEventListener('mousemove', onMouseMove);
            window.removeEventListener('mouseup', onMouseUp);
            headerBeingResized.classList.remove('header--being-resized');
            headerBeingResized = null;
        };

        // Get ready, they're about to resize
        const initResize = ({ target }) => {
            console.log('initResize');
            
            headerBeingResized = target.parentNode;
            window.addEventListener('mousemove', onMouseMove);
            window.addEventListener('mouseup', onMouseUp);
            headerBeingResized.classList.add('header--being-resized');
        };

        // Let's populate that columns array and add listeners to the resize handles
        document.querySelectorAll('th').forEach((header) => {
            const max = columnTypeToRatioMap[header.dataset.type] + 'fr';
            columns.push({ 
                header, 
                // The initial size value for grid-template-columns:
                size: `minmax(${min}px, ${max})`,
            });


            header.querySelector('.resize-handle')?.addEventListener('mousedown', initResize);
        });
    };

    componentDidUpdate() {
        this.initResizeEvents();
    }

    onFieldsetChange = (fs) => {
        this.setState({fieldsSelected: fs});
    }

    onSelectAllCheckboxes = (e) => {
        console.log("checkboxes selected")
    }

    onRowClick = (e) => {
        console.log("row clicked")

        var currentElement = e.target;

        if (currentElement.className == "material-icons" || currentElement.querySelector('.material-icons')) {
            return
        }
        
        while (currentElement.tagName != 'TR' && currentElement.parentElement !== null) {
            currentElement = currentElement.parentElement;
        }

        var checkbox = currentElement.querySelector('input[type=checkbox]');
        checkbox.checked = !checkbox.checked;
        //debugger
    }

    onEditClick = (e) => {
        console.log("edit")
    }

    onDeleteClick = (e) => {
        console.log("delete")
    }

    prepareMinMaxCssProps = () => {
        // Checkbox
        let tableStyle = {
            gridTemplateColumns: `
                minmax(60px, .5fr)
            `
        }

        if(this.state.data.length > 0) {
            Object.keys(this.state.fieldsSelected).forEach((el) => {
                if(this.state.fieldsSelected[el]) {
                    tableStyle.gridTemplateColumns += `
                    minmax(150px, 1.67fr)
                    `
                }
            });
        }

        //Edit + Delete
        tableStyle.gridTemplateColumns += `
                    minmax(60px, .5fr)
                    minmax(60px, .5fr)
                    `
        return tableStyle
    };

    render() {

        const tableStyle = this.prepareMinMaxCssProps();
        let that = this;

        const fieldsSelected = this.state.fieldsSelected;

        return (
            <>
            <table style={tableStyle}>
                <thead className="tableHead">
                    <tr>
                    <th><CheckboxList onChange={this.onSelectAllCheckboxes}/></th>
                    {
                        this.state.data.length > 0 &&
                        Object.keys(this.state.data[0]).map(function(key, index) {
                            if(fieldsSelected.hasOwnProperty(key) && fieldsSelected[key] === true) {
                                return <th key={index}>{key} <span className="resize-handle"></span></th>
                            }
                          })
                          
                    }
                    <th>{ /* Edit */ }</th>
                    <th>{ /* Delete */ }</th>
                    </tr>
                </thead>
                <tbody>
                    {this.state.data.map((row, rowIndex) => 
                        <tr key={rowIndex} onClick={that.onRowClick}>
                            <td><CheckboxList/></td>
                            {
                                Object.keys(row).map(function(key, index) {
                                    if(fieldsSelected.hasOwnProperty(key) && fieldsSelected[key] === true) {
                                        if(typeof components[fieldsSelected[key]] !== "undefined") {
                                            const CurrentComponent = components[fieldsSelected[key]];
                                            return (
                                                <td key={rowIndex + "_" + index}>
                                                    <CurrentComponent value={row[key]} /> 
                                                </td>
                                            )
                                        } else {
                                            return <td key={rowIndex + "_" + index}>{row[key]}</td>
                                        }
                                    }
                                 })

                                 //go json encoder is ordering maps lexicographically :(
                                // Object.keys(fs).map(function(key, index) {
                                //     if(typeof components[fs[key]] !== "undefined") {
                                //         const CurrentComponent = components[fs[key]];
                                //         return (
                                //             <td key={rowIndex + "_" + index}>
                                //                 <CurrentComponent value={row[key]} /> 
                                //             </td>
                                //         )
                                //     } else {
                                //         return <td key={rowIndex + "_" + index}>{row[key]}</td>
                                //     }
                                //   })
                            }
                            <td><span className="material-icons" onClick={this.onEditClick}>edit</span></td>
                            <td><span className="material-icons" onClick={this.onDeleteClick}>delete</span></td>
                        </tr>
                    )}
                </tbody>
            </table>
            <FieldsMenu fields={this.state.fieldsSelected} onFieldsetChange={this.onFieldsetChange}></FieldsMenu></>
        )
    }
}

export default StateList;