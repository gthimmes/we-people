import { FormEvent, useState } from "react";
import { useAuth } from "../auth";
import { ApiError } from "../api";

export default function Login() {
  const { login, register } = useAuth();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [slug, setSlug] = useState("acme-corp");
  const [orgName, setOrgName] = useState("");
  const [email, setEmail] = useState("admin@acme.test");
  const [password, setPassword] = useState("password123");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (mode === "login") await login(slug, email, password);
      else await register(orgName, email, password);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="center">
      <form className="card auth-card" onSubmit={submit}>
        <div className="brand big">
          We<span>People</span>
        </div>
        <p className="muted">
          {mode === "login" ? "Sign in to your organization" : "Create a new organization"}
        </p>

        {mode === "login" ? (
          <label>
            Organization slug
            <input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="acme-corp" />
          </label>
        ) : (
          <label>
            Organization name
            <input value={orgName} onChange={(e) => setOrgName(e.target.value)} placeholder="Acme Corp" required />
          </label>
        )}

        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>

        {error && <div className="error">{error}</div>}

        <button className="primary" disabled={busy}>
          {busy ? "…" : mode === "login" ? "Sign in" : "Create organization"}
        </button>

        <button
          type="button"
          className="link"
          onClick={() => {
            setMode(mode === "login" ? "register" : "login");
            setError("");
          }}
        >
          {mode === "login" ? "Create a new organization" : "I already have an account"}
        </button>
      </form>
    </div>
  );
}
