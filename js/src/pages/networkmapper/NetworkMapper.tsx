import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import NetworkTopologyDashboard, {
  NetworkTopologyData,
} from "./containers/NetworkTopologyDashboard";

// Admin-only Network Mapper page. Config.tsx gates it via requiresAdministration()
// (see Auth.canAccessModule), so non-admins never see it in the Pages menu and
// never mount it. The backend endpoint (/api/netmap/events) independently
// enforces admin, so the data is protected even if the route is hit directly.

interface NetworkMapperState {
  statusMessage: string;
  topology: NetworkTopologyData;
  scanning: boolean;
}

const initialTopology: NetworkTopologyData = {
  destination: "8.8.8.8",
  modem: { ip: "Idle", status: "Not started", latency: "-" },
  traceroute_hops: [],
  lan_devices: [],
};

export default class NetworkMapperPage extends PageComponent<{}, NetworkMapperState> {
  protected href = "netmapper";
  protected isPage = true;
  protected title = "Network Mapper";
  protected submenu = "tools";
  protected requiresAuth = true;
  protected requiresAdmin = true;

  private eventSource: EventSource | null = null;

  constructor(props: {}) {
    super(props);
    this.state = {
      statusMessage: "Idle. Press \u201cRun scan\u201d to map the network.",
      topology: initialTopology,
      scanning: false,
    } as NetworkMapperState;
  }

  // Config.tsx reads this to set the admin-only flag on the discovered page.
  public requiresAdministration(): boolean {
    return true;
  }

  componentWillUnmount() {
    this.eventSource?.close();
    this.eventSource = null;
  }

  private startScan = () => {
    this.eventSource?.close();
    this.setState({
      scanning: true,
      statusMessage: "Initializing network scanner...",
      topology: initialTopology,
    });

    const es = new EventSource("/api/netmap/events");
    this.eventSource = es;

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.setState((prev) => {
          const t = prev.topology;
          switch (data.step) {
            case "status":
              return { ...prev, statusMessage: data.payload };
            case "modem":
              return { ...prev, topology: { ...t, modem: data.payload } };
            case "hop_discovered":
              return { ...prev, topology: { ...t, traceroute_hops: [...t.traceroute_hops, data.payload] } };
            case "lan_discovered":
              return { ...prev, topology: { ...t, lan_devices: data.payload } };
            case "device_scanned":
              return {
                ...prev,
                topology: {
                  ...t,
                  lan_devices: t.lan_devices.map((dev) => (dev.ip === data.payload.ip ? data.payload : dev)),
                },
              };
            case "complete":
              es.close();
              this.eventSource = null;
              return { ...prev, statusMessage: "Scan complete.", scanning: false };
            default:
              return prev;
          }
        });
      } catch (e) {
        console.error("Failed to parse SSE message:", e);
      }
    };

    es.onerror = () => {
      es.close();
      this.eventSource = null;
      this.setState((prev) => ({
        ...prev,
        scanning: false,
        statusMessage:
          prev.statusMessage === "Scan complete."
            ? prev.statusMessage
            : "Connection closed (admin session required, or scan finished).",
      }));
    };
  };

  render() {
    const { statusMessage, topology, scanning } = this.state;
    return (
      <div className="container-fluid" style={{ paddingTop: 16 }}>
        <div className="d-flex justify-content-between align-items-center mb-2">
          <h1 className="h4 mb-0">Network Mapper</h1>
          <button className="btn btn-primary btn-sm" onClick={this.startScan} disabled={scanning}>
            {scanning ? "Scanning\u2026" : "Run scan"}
          </button>
        </div>

        <div
          className="mb-3"
          style={{
            background: "#0b2027",
            border: "1px solid #0e7490",
            color: "#67e8f9",
            padding: "6px 12px",
            fontFamily: "monospace",
            fontSize: 12,
            borderRadius: 6,
          }}
        >
          {scanning ? "\uD83D\uDFE2" : "\u26AA"} Live status: {statusMessage}
        </div>

        <NetworkTopologyDashboard data={topology} />
      </div>
    );
  }
}