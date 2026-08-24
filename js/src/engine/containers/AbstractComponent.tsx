import React from "react";
import { useLocation, useNavigate, useParams, Location, NavigateFunction, Params } from "react-router";

export interface WithRouterProps {
  location: Location;
  navigate: NavigateFunction;
  params: Params<string>;
}

export interface AbstractComponentProps {
  // Broadened component type to accept legacy React classes, modern components, and dynamic imports
  component: React.ComponentType<any> | React.ElementType<any> | any;
  [key: string]: any;
}

/**
 * Higher-Order Route Wrapper
 * Bridges React Router v7/v8 hooks into legacy Class-based components via props.
 */
export const AbstractComponent: React.FC<AbstractComponentProps> = ({ component: CurrentComponent, ...restProps }) => {
  const location = useLocation();
  const navigate = useNavigate();
  const params = useParams();

  // Ensure CurrentComponent is a valid React component before rendering
  if (!CurrentComponent) {
    console.error("AbstractComponent: props.component is undefined or null", restProps);
    return <div className="alert alert-danger m-3">Error: Component not found.</div>;
  }

  return (
    <CurrentComponent
      {...restProps}
      navigate={navigate}
      location={location}
      params={params}
    />
  );
};

export default AbstractComponent;