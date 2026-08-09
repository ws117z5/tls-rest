
import PageComponent from "@engine/PageComponent";
import {TextEdit} from "@engine/fields"

interface QueryExecutorProps {}
interface QueryExecutorState {}

class QueryExecutor extends PageComponent<QueryExecutorProps, QueryExecutorState> {

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
    return(
      <div>
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
        </div>
    )
  }
}