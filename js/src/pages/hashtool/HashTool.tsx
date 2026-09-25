import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/controllers/i18n";
import { hashAll, randomInput, HashResult } from "./hashes";

interface HashToolState {
  input: string;
  results: HashResult[];
  copied: string;
}

export default class HashTool extends PageComponent<{}, HashToolState> {
  protected isPage = true;
  protected href = "hash-tool";
  protected title = "Hash Generator";
  protected submenu = "tools";
  protected icon = "hash";
  protected requiresAuth = true;

  constructor(props: {}) {
    super(props);
    this.state = { input: "", results: [], copied: "" };
  }

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    this.recompute("");
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  private recompute = (input: string) => {
    this.setState({ input });
    hashAll(input).then((results) => this.setState({ results }));
  };

  private useRandom = () => {
    this.recompute(randomInput());
  };

  private copy = (name: string, value: string) => {
    navigator.clipboard?.writeText(value).then(() => {
      this.setState({ copied: name });
      setTimeout(() => this.setState((s) => (s.copied === name ? { copied: "" } : null)), 1200);
    });
  };

  render() {
    const { input, results, copied } = this.state;
    return (
      <div className="container-fluid pt-4">
        <h4 className="mb-3">{t("Hash Generator")}</h4>
        <div className="card">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-1">{t("Input")}</div>
            <div className="d-flex gap-2 mb-3">
              <input
                className="form-control"
                value={input}
                onChange={(e) => this.recompute(e.target.value)}
                placeholder={t("Type something to hash…")}
                spellCheck={false}
              />
              <button className="btn btn-outline-secondary text-nowrap" onClick={this.useRandom}>
                {t("Random")}
              </button>
            </div>

            <div className="text-muted small text-uppercase mb-1">{t("Results")}</div>
            <table className="table table-sm mb-0">
              <tbody>
                {results.map((r) => (
                  <tr key={r.name}>
                    <td className="text-nowrap" style={{ width: "12rem" }}>
                      <strong>{r.name}</strong>
                    </td>
                    <td>
                      <code style={{ wordBreak: "break-all" }}>{r.value}</code>
                    </td>
                    <td style={{ width: "5rem", textAlign: "right" }}>
                      <button
                        className="btn btn-sm btn-outline-secondary"
                        onClick={() => this.copy(r.name, r.value)}
                      >
                        {copied === r.name ? t("Copied!") : t("Copy")}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    );
  }
}
