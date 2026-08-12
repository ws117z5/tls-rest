import React, { useState, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Route, Routes, Navigate } from "react-router";
import axios from 'axios';

import { AbstractComponent } from '@controllers/AbstractComponent';
import Menu from "@containers/Menu";
import IndexPage from "@pages/IndexPage";
import Config from '@engine/Config';
import ModulePage, { ModeName } from '@engine/ModulePage';
import { ErrorBoundary } from "@pages/ErrorBoundary";

// Per-mode routes to register for a backend module. Only the modes the user is
// allowed (reported by /api/modules) get a route; the rest simply don't exist
// for them, and unknown URLs fall through to the catch-all.
const MODE_ROUTES: Array<{ mode: ModeName; suffix: string }> = [
  { mode: 'list', suffix: '' },
  { mode: 'create', suffix: '/create' },
  { mode: 'view', suffix: '/:id' },
  { mode: 'edit', suffix: '/:id/edit' },
];

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

          {/* Backend modules: one route per permitted mode, all rendered by the
              generic ModulePage (or a registered custom view). */}
          {Config.getModules().map((m) => {
            const base = '/' + m.href;
            return (
              <React.Fragment key={m.href}>
                {MODE_ROUTES.map(({ mode, suffix }) =>
                  m.modes.indexOf(mode) !== -1 ? (
                    <Route
                      key={mode}
                      path={base + suffix}
                      element={
                        <AbstractComponent
                          component={ModulePage}
                          module={m.name}
                          endpoint={m.href}
                          title={m.title}
                          mode={mode}
                          modes={m.modes}
                        />
                      }
                    />
                  ) : null
                )}
              </React.Fragment>
            );
          })}

          {/* Frontend-only custom pages. The home page (empty href) is served by
              the explicit "/" route above, so skip its route here — but it stays
              in the page list so the menu can link to it. */}
          {Config.getCustomPages().map((page, idx) => (
            page.href ? (
            <React.Fragment key={page.href || idx}>
              <Route
                path={'/' + (page.isPage ? "pages/" : "") + page.href}
                element={<AbstractComponent component={page.component} />}
              />
              {page.extraRoutes && page.extraRoutes.length > 0 &&
                page.extraRoutes.map((sub, subidx) => (
                  <Route
                    key={sub.href || (idx + "_" + subidx)}
                    path={sub.href}
                    element={<AbstractComponent component={sub.component} />}
                  />
                ))
              }
            </React.Fragment>
            ) : null
          ))}

          {/* Unknown paths fall through to the homepage. */}
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