import React, { useRef, useEffect } from "react";
import ForceGraph2D from "react-force-graph-2d";

// Live force-directed topology graph for the admin Network Mapper page. Ported
// from the standalone netmapper tool. Styling is inline (this app uses Bootstrap,
// not Tailwind); the graph itself is canvas-rendered so it needs no utility CSS.

export interface ModemInfo {
  ip: string;
  status: string;
  latency: string;
}

export interface HopInfo {
  ttl: number;
  ip: string;
  latency: string;
}

export interface DeviceNode {
  ip: string;
  type: string;
  open_ports: number[];
}

export interface NetworkTopologyData {
  destination: string;
  modem: ModemInfo;
  traceroute_hops: HopInfo[];
  lan_devices: DeviceNode[];
}

interface Props {
  data: NetworkTopologyData | null;
  height?: number;
}

export default function NetworkTopologyDashboard({ data, height = 640 }: Props) {
  const fgRef = useRef<any>(null);

  useEffect(() => {
    if (fgRef.current) {
      fgRef.current.d3Force("charge").strength(-150);
      fgRef.current.d3Force("link").distance(60);
    }
  }, [data]);

  if (!data) {
    return (
      <div style={{ padding: 48, background: "#0b0f14", color: "#22d3ee", fontFamily: "monospace" }}>
        Initializing Network Mesh Engine...
      </div>
    );
  }

  const nodes: any[] = [];
  const links: any[] = [];

  // 1. Destination (root).
  const destId = data.destination || "8.8.8.8";
  nodes.push({ id: destId, name: `Target: ${destId}`, group: "destination", val: 25, color: "#a855f7" });

  // 2. Traceroute hops.
  const hops = data.traceroute_hops ?? [];
  let prevNodeId = destId;
  hops.forEach((hop) => {
    const hopId = `hop-${hop.ip}`;
    nodes.push({ id: hopId, name: `Hop ${hop.ttl}: ${hop.ip} (${hop.latency})`, group: "wan", val: 15, color: "#6366f1" });
    links.push({ source: prevNodeId, target: hopId });
    prevNodeId = hopId;
  });

  // 3. Cable modem (coax boundary).
  const modemId = data.modem?.ip || "192.168.100.1";
  nodes.push({ id: modemId, name: `Modem (DOCSIS): ${modemId}`, group: "modem", val: 20, color: "#f59e0b" });
  links.push({ source: prevNodeId, target: modemId });

  // 4. Local gateway (LAN edge).
  const gatewayId = "192.168.1.1";
  nodes.push({ id: gatewayId, name: `Local Gateway: ${gatewayId}`, group: "gateway", val: 22, color: "#3b82f6" });
  links.push({ source: modemId, target: gatewayId });

  // 5. Local subnet devices.
  const devices = data.lan_devices ?? [];
  devices.forEach((dev) => {
    const devId = dev.ip;
    if (devId !== gatewayId) {
      nodes.push({
        id: devId,
        name: `${dev.ip}\n[${dev.open_ports?.length || 0} Ports Open]`,
        group: "device",
        val: 12,
        color: "#06b6d4",
      });
      links.push({ source: gatewayId, target: devId });
    }
  });

  return (
    <div style={{ position: "relative", background: "#030712", borderRadius: 8, overflow: "hidden" }}>
      <div style={{ position: "absolute", top: 12, left: 0, right: 0, textAlign: "center", zIndex: 10, pointerEvents: "none" }}>
        <div style={{ fontSize: 20, fontWeight: 800, color: "#22d3ee", letterSpacing: 1 }}>Live Network Mesh Graph</div>
        <div style={{ fontSize: 11, color: "#9ca3af", fontFamily: "monospace" }}>Drag nodes, scroll to zoom, hover to inspect</div>
      </div>

      <div style={{ width: "100%", height, cursor: "grab" }}>
        <ForceGraph2D
          ref={fgRef}
          graphData={{ nodes, links }}
          nodeLabel="name"
          nodeColor={(node: any) => node.color}
          nodeVal={(node: any) => node.val}
          linkColor={() => "#374151"}
          linkWidth={2}
          height={height}
          backgroundColor="#030712"
          nodeCanvasObject={(node: any, ctx, globalScale) => {
            const label = node.name;
            const fontSize = 12 / globalScale;
            ctx.font = `${fontSize}px monospace`;
            ctx.fillStyle = node.color;
            ctx.beginPath();
            ctx.arc(node.x, node.y, node.val / 2, 0, 2 * Math.PI, false);
            ctx.fill();
            ctx.lineWidth = 1.5;
            ctx.strokeStyle = "#ffffff";
            ctx.stroke();

            ctx.fillStyle = "#e5e7eb";
            ctx.textAlign = "center";
            ctx.textBaseline = "top";
            ctx.fillText(label, node.x, node.y + node.val / 2 + 4);
          }}
        />
      </div>
    </div>
  );
}