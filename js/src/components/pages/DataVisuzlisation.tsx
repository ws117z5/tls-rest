import React, { useRef, useState, ComponentProps } from "react";
import { Canvas, useFrame, ThreeElements } from "@react-three/fiber";
import { Mesh } from "three";

import PageComponent from "@engine/containers/PageComponent";
import OpenCV from "../../pages/opencv/containers/OpenCV";

interface CubeProps {
  position: [number, number, number];
  color: number | string;
}

function Cube({ position, color }: CubeProps) {
  const meshRef = useRef<Mesh>(null!);

  useFrame(() => {
    if (meshRef.current) {
      meshRef.current.rotation.x += 0.01;
      meshRef.current.rotation.y += 0.01;
    }
  });

  return (
    <mesh ref={meshRef} position={position}>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color={color} />
    </mesh>
  );
}

function Cubes() {
  const cubes: React.JSX.Element[] = [];
  for (let x = -1; x <= 1; x++) {
    for (let y = -1; y <= 1; y++) {
      for (let z = -1; z <= 1; z++) {
        cubes.push(
          <Cube
            key={`${x}-${y}-${z}`}
            position={[x * 2, y * 2, z * 2]}
            color={Math.random() * 0xffffff}
          />
        );
      }
    }
  }

  return (
    <Canvas>
      <ambientLight intensity={0.5} />
      <pointLight position={[10, 10, 10]} intensity={0.5} />
      {cubes}
    </Canvas>
  );
}

// Fixed: Using ThreeElements['mesh'] instead of MeshProps
function Box(props: ThreeElements['mesh']) {
  const meshRef = useRef<Mesh>(null!);
  const [hovered, setHover] = useState<boolean>(false);
  const [active, setActive] = useState<boolean>(false);

  useFrame((_state, delta) => {
    if (meshRef.current) {
      meshRef.current.rotation.x += delta;
    }
  });

  return (
    <mesh
      {...props}
      ref={meshRef}
      scale={active ? 1 : 1}
      onClick={() => setActive(!active)}
      onPointerOver={() => setHover(true)}
      onPointerOut={() => setHover(false)}
    >
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color={hovered ? "hotpink" : "orange"} />
    </mesh>
  );
}

function Grid(props: ComponentProps<typeof Canvas>) {
  const cubes: React.JSX.Element[] = [];
  for (let x = -1; x <= 1; x++) {
    for (let y = -1; y <= 1; y++) {
      for (let z = -1; z <= 1; z++) {
        cubes.push(
          <Cube
            key={`${x}-${y}-${z}`}
            position={[x * 2, y * 2, z * 2]}
            color={Math.random() * 0xffffff}
          />
        );
      }
    }
  }

  return (
    <Canvas {...props}>
      <ambientLight intensity={0.5} />
      <pointLight position={[10, 10, 10]} intensity={0.5} />
      {cubes}
    </Canvas>
  );
}

class DataVisualisation extends PageComponent {
  protected href = "dataviz";
  protected isPage = true;
  protected title = "Data Visualisation";

  render() {
    return (
      <div className="base">
        <textarea></textarea>
        <OpenCV />
      </div>
    );
  }
}

export default DataVisualisation;