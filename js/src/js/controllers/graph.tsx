import React, { useEffect, useRef, useState } from 'react';
import { Network } from 'vis-network/standalone';
import { DataSet } from 'vis-data/peer';
import 'vis-network/styles/vis-network.css';

type Node = { id: number; label: string };
type Edge = { from: number; to: number };

type GraphData = {
  nodes: Node[];
  edges: Edge[];
};

const visitedColor = '#FFC107';

const GraphTraversal: React.FC<{ data: GraphData }> = ({ data }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const networkRef = useRef<Network | null>(null);
  const [visited, setVisited] = useState<Set<number>>(new Set());
  const [nodesData, setNodesData] = useState<any>(null);
  const [edgesData, setEdgesData] = useState<any>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const visNodes = new DataSet(data.nodes);
    const visEdges = new DataSet(
      data.edges.map((edge, idx) => ({ ...edge, id: idx }))
    );

    setNodesData(visNodes);
    setEdgesData(visEdges);

    networkRef.current = new Network(containerRef.current, { nodes: visNodes, edges: visEdges }, {
      nodes: { shape: 'dot', size: 25, font: { size: 18 } },
      edges: { arrows: 'to' },
      physics: false,
    });
  }, [data]);

  const bfs = (start: number) => {
    const queue = [start];
    let i = 0;
    const visitedSet = new Set<number>();

    function step() {
      if (i >= queue.length) return;
      const nodeId = queue[i++];
      if (visitedSet.has(nodeId)) return step();

      visitedSet.add(nodeId);
      if (nodesData) nodesData.update({ id: nodeId, color: { background: visitedColor } });

      const neighbors = edgesData
        ?.get()
        .filter((e: any) => e.from === nodeId)
        .map((e: any) => e.to);

      for (const neighbor of neighbors) {
        if (!visitedSet.has(neighbor)) queue.push(neighbor);
      }

      setTimeout(step, 700);
    }

    step();
  };

  const startTraversal = () => {
    setVisited(new Set());
    nodesData?.forEach((n: any) => nodesData.update({ id: n.id, color: null }));
    bfs(0);
  };

  return (
    <div>
      <button onClick={startTraversal}>Start BFS Traversal</button>
      <div ref={containerRef} style={{ width: '100%', height: '90vh', border: '1px solid lightgray' }} />
    </div>
  );
};

export default GraphTraversal;