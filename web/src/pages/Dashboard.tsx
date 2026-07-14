import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, Approval, DashboardSummary } from "../api";
import { useAuth } from "../auth";

export default function Dashboard() {
  const { me } = useAuth();
  const [sum, setSum] = useState<DashboardSummary | null>(null);
  const [pending, setPending] = useState(0);

  useEffect(() => {
    api.get<DashboardSummary>("/dashboard").then(setSum).catch(() => setSum(null));
    api.get<{ data: Approval[] }>("/approvals").then((r) => setPending(r.data.length)).catch(() => setPending(0));
  }, []);

  const firstName = me?.email.split("@")[0];

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Welcome{firstName ? `, ${firstName}` : ""}</h1>
          <p className="muted">Here's what's happening in your organization</p>
        </div>
      </header>

      <div className="stat-row">
        <StatCard label="Headcount" value={sum?.headcount ?? "—"} to="/directory" />
        <StatCard label="Open positions" value={sum?.open_positions ?? "—"} to="/org" />
        <StatCard label="Departments" value={sum?.departments ?? "—"} to="/org" />
        <StatCard label="Awaiting you" value={pending} to="/time-off" highlight={pending > 0} />
      </div>

      <div className="detail-grid">
        <section className="card">
          <h3>Out today</h3>
          {!sum || sum.out_today.length === 0 ? (
            <p className="muted">Everyone's in today.</p>
          ) : (
            <ul className="list">
              {sum.out_today.map((o) => (
                <li key={o.worker_id}>
                  <span><strong>{o.name}</strong> <span className="muted small">· {o.leave_type}</span></span>
                  <span className="muted small">back {new Date(o.end_date).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="card">
          <h3>Recent hires</h3>
          {!sum || sum.recent_hires.length === 0 ? (
            <p className="muted">No one yet.</p>
          ) : (
            <ul className="list">
              {sum.recent_hires.map((h) => (
                <li key={h.worker_id}>
                  <span>
                    <Link to={`/people/${h.worker_id}`} className="plain-link"><strong>{h.name}</strong></Link>
                    {h.title && <span className="muted small"> · {h.title}</span>}
                  </span>
                  {h.hire_date && <span className="muted small">{new Date(h.hire_date).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}</span>}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}

function StatCard({ label, value, to, highlight }: { label: string; value: number | string; to: string; highlight?: boolean }) {
  return (
    <Link to={to} className={`stat-card ${highlight ? "stat-highlight" : ""}`}>
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </Link>
  );
}
