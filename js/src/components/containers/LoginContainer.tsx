import React, { useState } from "react";
import axios from "axios";
import AuthButton from "./AuthButton";
import "./LoginContainer.css";

interface LoginProps {
  name?: string;
}

type Mode = "login" | "register";

// OAuth providers, in display order. `key` is the backend route segment
// (/users/Auth/{key}Login); `icon` is served statically from /img/oauth.
const PROVIDERS: { key: string; label: string; icon: string }[] = [
  { key: "Google", label: "Continue with Google", icon: "/img/oauth/google.svg" },
  { key: "Github", label: "Continue with GitHub", icon: "/img/oauth/github.svg" },
  { key: "Facebook", label: "Continue with Facebook", icon: "/img/oauth/facebook.svg" },
  { key: "Vk", label: "Continue with VK", icon: "/img/oauth/vk.svg" },
];

// Login form: email/password (via /api/login, /api/register) plus OAuth sign-in.
// OAuth uses a full-page redirect to the backend flow (/users/Auth/{provider}),
// which establishes the session and redirects home.
const Login: React.FC<LoginProps> = () => {
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [userName, setUserName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const url = mode === "login" ? "/api/login" : "/api/register";
      const payload =
        mode === "login"
          ? { email, password }
          : { email, password, user_name: userName };
      await axios.post(url, payload);
      // Session cookie is set server-side; reload as the authenticated user.
      window.location.assign("/");
    } catch (err: any) {
      setError(err?.response?.data?.error || "Authentication failed");
      setLoading(false);
    }
  };

  const loginWith = (provider: string) =>
    window.location.assign(`/users/Auth/${provider}Login`);

  const switchMode = (m: Mode) => {
    setMode(m);
    setError("");
  };

  return (
    <div className="login-card">
      <h1 className="login-title">{mode === "login" ? "Sign in" : "Create account"}</h1>
      <p className="login-subtitle">
        {mode === "login" ? "Welcome back." : "Join in a few seconds."}
      </p>

      {error && <div className="login-error">{error}</div>}

      <form className="login-form" onSubmit={submit}>
        {mode === "register" && (
          <div className="login-field">
            <label className="login-label">Username</label>
            <input
              className="login-input"
              value={userName}
              onChange={(e) => setUserName(e.target.value)}
              autoComplete="username"
            />
          </div>
        )}

        <div className="login-field">
          <label className="login-label">Email</label>
          <input
            type="email"
            className="login-input"
            value={email}
            required
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </div>

        <div className="login-field">
          <label className="login-label">Password</label>
          <input
            type="password"
            className="login-input"
            value={password}
            required
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === "login" ? "current-password" : "new-password"}
          />
        </div>

        <button type="submit" className="login-submit" disabled={loading}>
          {loading ? "Please wait\u2026" : mode === "login" ? "Sign in" : "Register"}
        </button>
      </form>

      <div className="login-divider"><span>or</span></div>

      <div className="oauth-list">
        {PROVIDERS.map((p) => (
          <AuthButton key={p.key} name={p.label} icon={p.icon} call={() => loginWith(p.key)} />
        ))}
      </div>

      <div className="login-switch">
        {mode === "login" ? (
          <button type="button" className="login-link" onClick={() => switchMode("register")}>
            Need an account? <strong>Register</strong>
          </button>
        ) : (
          <button type="button" className="login-link" onClick={() => switchMode("login")}>
            Have an account? <strong>Sign in</strong>
          </button>
        )}
      </div>
    </div>
  );
};

export default Login;

