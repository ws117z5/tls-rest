import React from "react";

interface ErrorBoundaryState {
  hasError: boolean;
  error: any;
}

export default class ErrorBoundary extends React.Component<React.PropsWithChildren<{}>, ErrorBoundaryState> {
  constructor(props: {}) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: any) {
    return { hasError: true, error };
  }

  componentDidCatch(error: any, errorInfo: any) {
    // You can log error info here if needed
    console.error("ErrorBoundary caught an error", error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return <div style={{ color: "red" }}>Something went wrong: {String(this.state.error)}</div>;
    }
    return this.props.children;
  }
}