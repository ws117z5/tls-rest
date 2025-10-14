import React from "react";
import PageComponent from "../../controllers/PageComponent";
import {TextEdit} from "../container/Fields"

interface IndexPageProps {}
interface IndexPageState {}

class IndexPage extends PageComponent<IndexPageProps, IndexPageState> {
  static title = "Home";

  constructor(props: IndexPageProps) {
    super(props);
  }

  componentWillMount() {
    // Add any logic needed before the component mounts
  }

  componentDidMount() {
    const canvas = document.querySelector<HTMLCanvasElement>("#webgl");

    // Initialize the GL context
    if (!canvas) {
      console.log("Unable to find canvas element.");
      return;
    }

    const gl = canvas.getContext("webgl");

    // Only continue if WebGL is available and working
    if (!gl) {
      console.log(
        "Unable to initialize WebGL. Your browser or machine may not support it."
      );
      return;
    }

    // Add WebGL initialization logic here
  }

  //tmp: function to sned query to database
  sendQuery = () => {
    // This function should send a query to the database
    // endpoint is loaclhost:8080/dbquery
    const query = (document.querySelector<HTMLInputElement>("#query") || { value: "" }).value;
    if (!query) {
      console.log("Query is empty");
      return;
    }
    fetch("http://localhost:8080/dbquery", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ query }),
    })
      .then((response) => response.json())
      .then((data) => {
        console.log("Query result:", data);
      })
      .catch((error) => {
        console.error("Error sending query:", error);
      });
  };


  render() {
    return (
      <div className="base">
        <div style={webGLDiv}>
          <canvas
            id="webgl"
            style={webGL}
            width="1200"
            height="600"
          ></canvas>
        </div>
        
       <TextEdit
          id="query"
          label="Query"
          placeholder="Enter your query here"
          width="300px" />

        <button
          onClick={this.sendQuery}
          style={{
            width: "100px",
            padding: "10px",
            backgroundColor: "#007bff",
            color: "#fff",
            border: "none",
            borderRadius: "5px",
            cursor: "pointer",
          }}
        > Send Query </button>


        <div style={parallax}>
          <div id="group1" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 1</h1>
              </div>
              <div className="title">Base Layer</div>
              <div>
                <h1>Hello World,</h1>
                <h1>this is Vladimir Koroteev page</h1>
                <h1>Enjoy</h1>
              </div>
            </div>
          </div>

          <div id="group2" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 2</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
            <div style={{ ...parallax__layer, ...parallax__layer__back }}>
              <div className="title">
                <h1>Group 2</h1>
              </div>
              <div className="title">Background Layer</div>
            </div>
          </div>

          <div id="group3" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__fore }}>
              <div className="title">
                <h1>Group 3</h1>
              </div>
              <div className="title">Foreground Layer</div>
            </div>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 3</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
          </div>

          <div id="group4" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 4</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
            <div style={{ ...parallax__layer, ...parallax__layer__back }}>
              <div className="title">
                <h1>Group 4</h1>
              </div>
              <div className="title">Background Layer</div>
            </div>
            <div
              style={{
                ...parallax__layer,
                ...parallax__layer__deep,
                ...parallax__layer__deep__img,
              }}
            >
              <div className="title">
                <h1>Group 4</h1>
              </div>
              <div className="title">Deep Background Layer</div>
            </div>

            <div style={{ ...parallax__layer, ...parallax__layer__fore }}>
              <div className="title">
                <h1>Group 4</h1>
              </div>
              <div className="title">Foreground Layer</div>
            </div>
          </div>

          <div id="group5" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 5</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
          </div>

          <div id="group6" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__back }}>
              <div className="title">
                <h1>Group 6</h1>
              </div>
              <div className="title">Background Layer</div>
            </div>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 6</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
          </div>

          <div id="group7" style={parallax__group}>
            <div style={{ ...parallax__layer, ...parallax__layer__base }}>
              <div className="title">
                <h1>Group 7</h1>
              </div>
              <div className="title">Base Layer</div>
            </div>
          </div>
        </div>
      </div>
    );
  }
}

/**
 * Styles block
 */

const webGL: React.CSSProperties = {
  color: "blue",
  width: "100%",
  height: "600px",
};

const webGLDiv: React.CSSProperties = {
  position: "absolute",
  zIndex: 0,
  width: "100%",
  height: "100%",
  padding: 0,
  margin: 0,
};

const parallax: React.CSSProperties = {
  height: "100vh",
  overflowX: "hidden",
  overflowY: "auto",
  WebkitPerspective: "300px",
  perspective: "300px",
};

const parallax__group: React.CSSProperties = {
  position: "relative",
  height: "100vh",
  WebkitTransformStyle: "preserve-3d",
  transformStyle: "preserve-3d",
  WebkitTransition: "-webkit-transform 0.5s",
  transition: "transform 0.5s",
};

const parallax__layer: React.CSSProperties = {
  position: "absolute",
  top: 0,
  left: 0,
  right: 0,
  bottom: 0,
  backgroundColor: "#fff",
};

const parallax__layer__fore: React.CSSProperties = {
  WebkitTransform: "translateZ(90px) scale(.7)",
  transform: "translateZ(90px) scale(.7)",
  zIndex: 1,
};

const parallax__layer__base: React.CSSProperties = {
  WebkitTransform: "translateZ(0)",
  transform: "translateZ(0)",
  zIndex: 4,
};

const parallax__layer__back: React.CSSProperties = {
  WebkitTransform: "translateZ(-300px) scale(2)",
  transform: "translateZ(-300px) scale(2)",
  zIndex: 3,
};

const parallax__layer__deep: React.CSSProperties = {
  WebkitTransform: "translateZ(-600px) scale(3)",
  transform: "translateZ(-600px) scale(3)",
  zIndex: 2,
};

const parallax__layer__deep__img: React.CSSProperties = {
  background: '#000 url("/img/background.jpg") no-repeat',
  backgroundSize: "cover",
  color: "#fff",
};

export default IndexPage;