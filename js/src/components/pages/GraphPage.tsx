'use strict';

import React from "react";
import PageComponent from "@engine/PageComponent";
import FunctionGraph from "@containers/Graph/FunctionGraph";

interface GraphFunction {
  fn: (x: number, additionalParams?: Record<string, any>) => number;
  additionalParams?: Record<string, any>;
  latex: string;
}

class GraphPage extends PageComponent<{}, {}> {
  protected href = "graph";
  protected isPage = true;
  protected title = "Graphs";
  protected submenu = "tools";

  constructor(props: {}) {
    super(props);
  }

  componentWillMount() {
    // Example API request placeholder
    // axios.get(`https://localhost:8080/users/${this.props.subreddit}`)
    // Request.apiRequest('users', this)
  }

  render() {
    const functions: GraphFunction[] = [
      {
        fn: (x) => x * x,
        latex: "x^2",
      },
      {
        fn: (x, additionalParams) => {
          const { mu = 0, sigma = 1 } = additionalParams || {};
          // Standard normal distribution formula
          return (
            Math.exp((-0.5) * Math.pow((x - mu) / sigma, 2)) /
            (sigma * Math.sqrt(2 * Math.PI))
          );
        },
        additionalParams: { mu: 0, sigma: 1 },
        latex:
          "\\frac{1}{\\sigma \\sqrt{2\\pi}}e^{-\\frac{1}{2} \\left( \\frac{x-\\mu}{\\sigma} \\right)^2}",
      },
    ];

    return (
      <div className="base">
        This is graphs
        <FunctionGraph fns={functions} />
      </div>
    );
  }
}

export default GraphPage;