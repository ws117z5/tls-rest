import React from 'react';
import { useFrame } from '@react-three/fiber';

import PageComponent from '../../controllers/PageComponent';

function Cube({ position, color }) {
  const meshRef = React.useRef();

  useFrame(() => {
    meshRef.current.rotation.x += 0.01;
    meshRef.current.rotation.y += 0.01;
  });

  return (
    <mesh ref={meshRef} position={position}>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color={color} />
    </mesh>
  );
}

function Cubes({}) {
    const cubes = [];
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

function Box(props) {
    // This reference will give us direct access to the mesh
    const meshRef = useRef()
    // Set up state for the hovered and active state
    const [hovered, setHover] = useState(false)
    const [active, setActive] = useState(false)
    // Subscribe this component to the render-loop, rotate the mesh every frame
    useFrame((state, delta) => (meshRef.current.rotation.x += delta))
    // Return view, these are regular three.js elements expressed in JSX
    return (
      <mesh
        {...props}
        ref={meshRef}
        scale={active ? 1 : 1}
        onClick={(event) => setActive(!active)}
        onPointerOver={(event) => setHover(true)}
        onPointerOut={(event) => setHover(false)}>
        <boxGeometry args={[1, 1, 1]} />
        <meshStandardMaterial color={hovered ? 'hotpink' : 'orange'} />
      </mesh>
    )
}

function Grid(props) {
    const cubes = [];
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


class DataVisualisation extends PageComponent {
    static href = 'dataviz'
    static isPage = true;
    static title = 'Data Visualisation'
    
    render() {
      return (
        <div className="base">
          <textarea></textarea>
          <OpenCV></OpenCV>
        </div>
      )
    }
  }
  
  export default OpenCVPage;


