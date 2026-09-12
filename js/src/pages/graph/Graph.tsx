'use strict';

import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import FunctionGraph from "./containers/FunctionGraph";
import { t, subscribe } from "@engine/i18n";

interface GraphFunction {
  fn: (x: number, additionalParams?: Record<string, any>) => number;
  additionalParams?: Record<string, any>;
  latex?: string;
  label?: string;
  color?: string;
}

const DEFAULT_FUNCTIONS: GraphFunction[] = [
  {
    fn: (x) => x * x,
    latex: "x^2",
    color: "rgba(13, 110, 253, 1)",
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
    color: "rgba(220, 53, 69, 1)",
  },
];

// Cycled as functions are added, so each new plot gets a distinct color.
const PALETTE = [
  "rgba(13, 110, 253, 1)",
  "rgba(220, 53, 69, 1)",
  "rgba(25, 135, 84, 1)",
  "rgba(255, 193, 7, 1)",
  "rgba(111, 66, 193, 1)",
  "rgba(13, 202, 240, 1)",
];

interface GraphPageState {
  functions: GraphFunction[];
  expression: string;
  error: string;
}

class GraphPage extends PageComponent<{}, GraphPageState> {
  protected href = "graph";
  protected isPage = true;
  protected title = "Graphs";
  protected icon = "graphs";
  protected submenu = "tools";

  constructor(props: {}) {
    super(props);
    this.state = {
      ...this.state,
      functions: DEFAULT_FUNCTIONS,
      expression: "",
      error: "",
    } as GraphPageState;
  }

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  // Same pattern as the Array Iterator tool: the visitor's own typed
  // expression, evaluated in their own browser via Function — nothing is sent
  // server-side or shown to any other visitor, so this is no different from
  // them opening their own JS console.
  private addFunction = (e: React.FormEvent) => {
    e.preventDefault();
    const expr = this.state.expression.trim();
    if (!expr) return;
    try {
      // eslint-disable-next-line no-new-func
      const fn = new Function("x", `return (${expr});`) as (x: number) => number;
      const probe = fn(1);
      if (typeof probe !== "number" || Number.isNaN(probe)) {
        throw new Error(t("Expression did not evaluate to a number"));
      }
      const color = PALETTE[this.state.functions.length % PALETTE.length];
      this.setState((s) => ({
        functions: [...s.functions, { fn, label: expr, color }],
        expression: "",
        error: "",
      }));
    } catch (err: any) {
      this.setState({ error: `${t("Error:")} ${err.message}` });
    }
  };

  private removeFunction = (index: number) => {
    this.setState((s) => ({ functions: s.functions.filter((_, i) => i !== index) }));
  };

  render() {
    const { functions, expression, error } = this.state;
    return (
      <div className="base p-3">
        <h1 className="h4 mb-3">{t("Graphs")}</h1>

        <form className="d-flex flex-wrap gap-2 align-items-start mb-2" onSubmit={this.addFunction}>
          <input
            type="text"
            className="form-control"
            style={{ maxWidth: 320 }}
            placeholder={t("Enter a function of x, e.g. Math.sin(x) * 2")}
            value={expression}
            onChange={(e) => this.setState({ expression: e.target.value })}
          />
          <button type="submit" className="btn btn-primary" disabled={!expression.trim()}>
            {t("Add function")}
          </button>
        </form>
        {error && <div className="text-danger small mb-2">{error}</div>}

        <ul className="list-unstyled mb-3">
          {functions.map((f, i) => (
            <li key={i} className="d-flex align-items-center gap-2 mb-1">
              <span
                style={{
                  display: "inline-block",
                  width: 10,
                  height: 10,
                  borderRadius: "50%",
                  backgroundColor: f.color || "#000",
                }}
              />
              <code className="small">{f.label || f.latex}</code>
              <button
                type="button"
                className="btn btn-sm btn-link text-danger p-0"
                onClick={() => this.removeFunction(i)}
              >
                {t("remove")}
              </button>
            </li>
          ))}
        </ul>

        <FunctionGraph fns={functions} />
      </div>
    );
  }
}

export default GraphPage;
