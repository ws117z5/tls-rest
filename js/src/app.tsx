import React, { useState, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes, Navigate } from "react-router";
import axios from 'axios';

import { AbstractComponent } from '@controllers/AbstractComponent';
import Menu from "@containers/Menu";
import IndexPage from "@pages/IndexPage";
import Config from '@controllers/config';
import Auth from '@controllers/auth';
import { ErrorBoundary } from "@pages/ErrorBoundary";

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
              // Don't register routes the current user can't access (auth, admin,
              // or per-module rights) — direct URLs fall through to the catch-all.
              if (!Auth.canAccessModule(module)) {
                return null;
              }
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
          {/* Unknown paths (e.g. a browser hitting the /papers API endpoint,
              which has no page) fall through to the homepage. */}
          <Route path="*" element={<Navigate to="/" replace />} />
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