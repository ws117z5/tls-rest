import React, { Component } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";


//basic page configuration 
//router -> abstract component (functional with hooks) -> class component recieving hooks
export function AbstractComponent(props, state) {
    const location = useLocation();
    const navigate = useNavigate();
    const params = useParams();

    const CurrentComponent = props.component;

    // Ensure CurrentComponent is a valid component
    if (!CurrentComponent) {
        // Handle the case where the component is not provided, e.g., return null or an error message
        console.error("AbstractComponent: props.component is undefined or null", props);
        return React.createElement("div", null, "Error: Component not found.");
    }
    

    return React.createElement(CurrentComponent, {...props, navigate: navigate, location: location, params: params });

  }