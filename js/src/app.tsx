import React, { useState, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes } from "react-router";
import axios from 'axios';

import { AbstractComponent } from './js/controllers/AbstractComponent';
import Menu from "./js/components/containers/Menu";
import { IndexPage } from "./js/components/pages";
import Config from './js/controllers/config';
import { ErrorBoundary } from "./js/components/pages/ErrorBoundary";

const App: React.FC = () => {
  const [loaded, setLoaded] = useState<boolean>(false);

  useEffect(() => {
    axios.defaults.headers.common['X-Request-Type'] = 'api';
    Config.init().then(() => {
      setLoaded(true);
    });
  }, []);

  if (!loaded) {
    return <></>;
  }

  return (
    <div>
      <Menu />
      <ErrorBoundary>
        <Routes>
          <Route path='/' element={<IndexPage />} />
          {
            Config.getAll()?.map((module, idx) => {
              return (
                <React.Fragment key={module.href || idx}>
                  <Route 
                    path={'/' + (module.isPage ? "pages/" : "") + module.href} 
                    element={<AbstractComponent component={module.component} />} 
                  />
                  {module.extraRoutes && module.extraRoutes.length > 0 &&
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
  );
};

const rootElement = document.getElementById('root');

if (rootElement) {
  const root = createRoot(rootElement);
  root.render(
    <React.StrictMode>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </React.StrictMode>
  );
}