import React from "react";
import PageComponent from "@engine/PageComponent";

interface ArrayIteratorState {
  items: string[];
  loopLogic: string;
  currentIndex: number;
  iterator: ((currentIndex: number, length: number, direction: string) => number) | null;
  error: string;
}

export default class ArrayIterator extends PageComponent<{}, ArrayIteratorState> {
  protected isPage = true;
  protected href = "arrayiter";
  protected title = "Array Iterator";

  constructor(props: {}) {
    super(props);

    this.state = {
      items: [],
      loopLogic: "",
      currentIndex: 0,
      iterator: null,
      error: "",
    };
  }

  handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const { loopLogic } = this.state;

    try {
      // Parse items as an array
      const parsedItems = JSON.parse(this.state.items as unknown as string);
      if (!Array.isArray(parsedItems)) {
        throw new Error("Items must be an array");
      }

      // Create the custom iterator function
      const customIterator = new Function(
        "currentIndex",
        "length",
        "direction",
        `return (${loopLogic});`
      ) as (currentIndex: number, length: number, direction: string) => number;

      this.setState({
        items: parsedItems,
        iterator: customIterator,
        currentIndex: 0,
        error: "",
      });
    } catch (err: any) {
      this.setState({ error: `Error: ${err.message}` });
    }
  };

  next = () => {
    const { iterator, items } = this.state;
    if (iterator) {
      this.setState((prevState) => ({
        currentIndex: iterator(prevState.currentIndex, items.length, "next"),
      }));
    }
  };

  prev = () => {
    const { iterator, items } = this.state;
    if (iterator) {
      this.setState((prevState) => ({
        currentIndex: iterator(prevState.currentIndex, items.length, "prev"),
      }));
    }
  };

  render() {
    const { items, loopLogic, currentIndex, error } = this.state;

    return (
      <div className="flex flex-col items-center space-y-6">
        {/* Form for input */}
        <form onSubmit={this.handleSubmit} className="flex flex-col space-y-4">
          <input
            type="text"
            placeholder='Enter array like ["A", "B", "C", "D"]'
            value={items as unknown as string}
            onChange={(e) => this.setState({ items: e.target.value.split(",") })}
            className="w-96 p-2 border rounded"
          />
          <input
            type="text"
            placeholder="Enter loop logic (e.g., (currentIndex + 1) % length)"
            value={loopLogic}
            onChange={(e) => this.setState({ loopLogic: e.target.value })}
            className="w-96 p-2 border rounded"
          />
          <button
            type="submit"
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
          >
            Start Loop
          </button>
          {error && <div className="text-red-500">{error}</div>}
        </form>

        {/* Visualization */}
        {items.length > 0 && (
          <>
            <div className="flex space-x-2">
              {items.map((item, index) => (
                <div
                  key={index}
                  className={`p-2 border rounded ${
                    index === currentIndex
                      ? "bg-blue-500 text-white"
                      : "bg-gray-200 text-black"
                  }`}
                >
                  {item}
                </div>
              ))}
            </div>

            {/* Navigation buttons */}
            <div className="flex space-x-2">
              <button
                onClick={this.prev}
                className="px-4 py-2 bg-gray-300 rounded hover:bg-gray-400"
              >
                Previous
              </button>
              <button
                onClick={this.next}
                className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
              >
                Next
              </button>
            </div>

            {/* Current Index */}
            <div>
              Current Index:{" "}
              <span className="font-bold text-blue-500">{currentIndex}</span>
            </div>
          </>
        )}
      </div>
    );
  }
}