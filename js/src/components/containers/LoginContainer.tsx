import React, { useState } from "react";
import axios from "axios";
import AuthButton from "./AuthButton";

interface LoginProps {
  name?: string;
}

type Mode = "login" | "register";

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

  const loginGoogle = () => window.location.assign("/users/Auth/GoogleLogin");

  const switchMode = (m: Mode) => {
    setMode(m);
    setError("");
  };

  return (
    <div className="login-container" style={{ maxWidth: 380, margin: "2rem auto" }}>
      <h1 className="h4 mb-3">{mode === "login" ? "Sign in" : "Create account"}</h1>

      {error && <div className="alert alert-danger">{error}</div>}

      <form onSubmit={submit}>
        {mode === "register" && (
          <div className="form-group mb-2">
            <label className="form-label">Username</label>
            <input
              className="form-control"
              value={userName}
              onChange={(e) => setUserName(e.target.value)}
              autoComplete="username"
            />
          </div>
        )}

        <div className="form-group mb-2">
          <label className="form-label">Email</label>
          <input
            type="email"
            className="form-control"
            value={email}
            required
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Password</label>
          <input
            type="password"
            className="form-control"
            value={password}
            required
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === "login" ? "current-password" : "new-password"}
          />
        </div>

        <button type="submit" className="btn btn-primary w-100" disabled={loading}>
          {loading ? "Please wait..." : mode === "login" ? "Sign in" : "Register"}
        </button>
      </form>

      <div className="text-center my-2 text-muted">or</div>

      <ul className="list-unstyled mb-0">
        <li>
          <AuthButton name="Continue with Google" call={loginGoogle} />
        </li>
      </ul>

      <div className="text-center mt-3">
        {mode === "login" ? (
          <button type="button" className="btn btn-link p-0" onClick={() => switchMode("register")}>
            Need an account? Register
          </button>
        ) : (
          <button type="button" className="btn btn-link p-0" onClick={() => switchMode("login")}>
            Have an account? Sign in
          </button>
        )}
      </div>
    </div>
  );
};

export default Login;