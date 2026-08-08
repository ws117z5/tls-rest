import React, { Component, ChangeEvent } from "react";
import Input from "../presentational/Input";

interface RunningLettersProps {
  children: string;
  style?: React.CSSProperties;
  action?: () => void;
}

interface RunningLettersState {
  hasError: boolean;
  string: string;
  [key: string]: any;
}

class RunningLetters extends Component<
  RunningLettersProps,
  RunningLettersState
> {
  protected initialString = "";
  constructor(props) {
    super(props);
    this.handleChange = this.handleChange.bind(this);

    //"".charCodeAt(index)

    this.state = {
      string: RunningLetters.randomizeString(props.children, []),
      hasError: typeof this.props.children !== "string" ? true : false,
    };

    //this.initialString = this.props.children;
    if (typeof this.props.children === "string") {
      this.initialString = this.props.children;
    }
    //console.log(this.props.children);
  }

  //Randomizses string starting with position
  static randomizeString(string, position) {
    return Array.prototype.map
      .call(string, (el, i) => {
        //var ci = string.charCodeAt(i);

        if (!allChars.includes(el)) {
          console.error("unrecognized char code:" + el.charCodeAt(0));
        } else {
          let current;
          switch (true) {
            case lowercase.includes(el):
              current = lowercase;
              break;

            case uppercase.includes(el):
              current = uppercase;
              break;

            case specialChars.includes(el):
              current = specialChars;
              break;

            /* faster, fragmented version

                case lowercase_one.includes(el):
                    current = lowercase_one;
                    break;

                case lowercase_two.includes(el):
                    current = lowercase_two;
                    break;

                case uppercase_one.includes(el):
                    current = uppercase_one;
                    break;

                case uppercase_two.includes(el):
                    current = uppercase_two;
                    break;

                case specialChars_one.includes(el):
                    current = specialChars_one;
                    break;

                case specialChars_two.includes(el):
                    current = specialChars_two;
                    break;
                    */
          }
          //let current = specialChars.includes(el) ? specialChars : lowercase.includes(el) ? lowercase : uppercase;

          if (position.includes(i)) {
            return el;
          } else {
            return current[Math.floor(Math.random() * current.length)];

            //acsii logic, too slow
            //return String.fromCharCode(Math.floor(Math.random() * (128 - 33) + 33));
          }
        }
      })
      .join("");
  }

  handleChange(event) {
    this.setState({ [event.target.id]: event.target.value });
  }

  componentDidMount() {
    var start;
    var that = this;

    var POSITION: number[] = [];
    var newString;

    let framesPerSecond = 40;

    function blink(timestamp) {
      setTimeout(function () {
        //if (start === undefined) start = timestamp;

        //const elapsed = timestamp - start;

        newString = RunningLetters.randomizeString(that.state.string, POSITION);

        for (var i = 0; i < that.initialString.length; i++) {
          if (newString[i] == that.initialString[i]) {
            POSITION.push(i);
          }
          //alert(str.charAt(i));
        }

        that.setState({ string: newString });

        if (that.state.string != that.initialString) {
          // Stop the animation after 2 seconds
          requestAnimationFrame(blink);
        } else {
          if (typeof that.props.action === "function") {
            that.props.action();
          }
        }
      }, 1000 / framesPerSecond);
    }

    requestAnimationFrame(blink);
  }

  render() {
    //const { seo_title } = this.state;
    if (this.state.hasError) {
      return "";
    } else {
      return <span style={this.props.style}>{this.state.string}</span>;
    }
  }
}

//one, two, reducing sample space to increase speed
//this way we can manipulate actual speed with frame speed

const specialChars = " !\"#$%^&*()_+',./:;<>=?@\\[]-{}`~|\n\t\r".split("");

const specialChars_one = " !\"#$%^&*()_+',./".split("");
const specialChars_two = ":;<>=?@\\[]-{}`~|\n\t\r".split("");

const lowercase = "abcdefghijklmnopqrstuvwxyz".split("");

const lowercase_one = "abcdefghijklm".split("");
const lowercase_two = "nopqrstuvwxyz".split("");

const uppercase = "abcdefghijklmnopqrstuvwxyz".toUpperCase().split("");

const uppercase_one = "abcdefghijklm".toUpperCase().split("");
const uppercase_two = "nopqrstuvwxyz".toUpperCase().split("");

const allChars = [...specialChars, ...lowercase, ...uppercase];

export default RunningLetters;