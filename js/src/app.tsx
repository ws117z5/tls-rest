import React, { Component } from 'react'
import ReactDOM from 'react-dom';
import { BrowserRouter, Route, Routes, RouteProps } from "react-router-dom";

import 'bootstrap/dist/css/bootstrap.min.css';
import { AbstractComponent} from './js/controllers/AbstractComponent';


//import _ from 'lodash'; TODO why?
//import FormContainer from "./js/components/container/FormContainer.jsx";

import axios from 'axios';

import Menu from "./js/components/container/Menu";
import Clock from "./js/components/container/Clock/Clock";

import { 
  IndexPage, 
  GraphPage, 
  LoginPage, 
  ProfilePage, 
  UsersPage, 
  PostsPage, 
  DataPage, 
  MoneyPage, 
  OpenCVPage, 
  ImageProcessingPage 
} from "./js/components/pages";

import Config from './js/controllers/config';
import { ErrorBoundary } from "./js/components/pages/ErrorBoundary";

//import modules from './modules.json'

type MyProps = { };
type MyState = { loaded: boolean };

class App extends React.Component<MyProps, MyState> {

  constructor(props: any) {
    super(props);

    this.state = {
      loaded: false
    }
  }

  componentDidMount() {
    //axios.get(`https://localhost:8080/users/${this.props.subreddit}`)
    /*
    axios.get(`https://localhost/sessions`)
        .then(res => {
            //console.log(res.data)
            //const users = res.data.children.map(obj => obj.data);
            //this.setState({ users: res.data });
        });
    }
    */

    axios.defaults.headers.common['X-Request-Type'] = 'api';
    Config.init().then(() => {
      this.setState({loaded: true})
    });
  }


  render() {

    const { loaded } = this.state;
    const GraphInfo = Config.get("GraphPage");

    return !loaded ? <></> :(
      <div>
        <Menu />
        <Clock />
        <ErrorBoundary>
          <Routes>
            <Route path='/' element={<IndexPage/>} />
            {
                Config.getAll()?.map((module, idx) => {
                  return (
                    <React.Fragment key={module.href || idx}>
                      <Route path={'/' + (module.isPage ? "pages/" : "") + module.href} element={<AbstractComponent component={module.component} />} />
                      {module.extraRoutes.length > 0 &&
                        module.extraRoutes.map((submodule, subidx) => (
                          <Route
                            path={submodule.href}
                            key={submodule.href || (idx + "_" + subidx)}
                            element={<AbstractComponent component={submodule.component} />}
                          />
                        ))
                      }
                    </React.Fragment>
                  );
                })
            }
          </Routes>
        </ErrorBoundary>
      </div>
    )
  }
}

const rootElement = document.getElementById('root');
// Create a ES6 class component

// Use the ReactDOM.render to show your component on the browser
ReactDOM.render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
  rootElement
);