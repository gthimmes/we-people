import { useEffect, useState } from "react";
import { api, Bucket, ReportData } from "../api";

export default function Analytics() {
  const [r, setR] = useState<ReportData | null>(null);

  useEffect(() => {
    api.get<ReportData>("/reports").then(setR).catch(() => setR(null));
  }, []);

  if (!r) return <div className="muted">Loading…</div>;

  const money = (n: number) => new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(n);

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Analytics</h1>
          <p className="muted">People metrics across your organization</p>
        </div>
      </header>

      <div className="stat-row">
        <Tile value={r.headcount} label="Active headcount" />
        <Tile value={`${Math.round(r.turnover_rate * 100)}%`} label="Turnover (12mo)" />
        <Tile value={r.time_off.pending_requests} label="Pending time off" />
        <Tile value={`${Math.round(r.time_off.balance_liability_hours)}h`} label="Leave liability" />
      </div>

      <div className="detail-grid">
        <BarCard title="Headcount by department" buckets={r.headcount_by_department} />
        <BarCard title="Headcount by location" buckets={r.headcount_by_location} />
      </div>

      <div className="detail-grid">
        <BarCard title="Gender" buckets={r.gender_breakdown} />
        <BarCard title="Employment status" buckets={r.status_breakdown} />
      </div>

      {r.compensation && (
        <section className="card">
          <div className="panel-head">
            <h3>Compensation</h3>
            <span className="muted small">avg {money(r.compensation.avg_annual)} / yr</span>
          </div>
          <BarRows
            rows={r.compensation.by_department.map((b) => ({ label: `${b.label} (${b.count})`, count: b.avg }))}
            format={money}
          />
        </section>
      )}

      {r.monthly_trend.length > 0 && (
        <section className="card">
          <h3>Hires vs terminations (12 months)</h3>
          <ul className="list">
            {r.monthly_trend.map((m) => (
              <li key={m.month}>
                <span className="mono">{m.month}</span>
                <span className="muted small">{m.hires} hired · {m.terminations} left</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function Tile({ value, label }: { value: number | string; label: string }) {
  return (
    <div className="stat-card" style={{ cursor: "default" }}>
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  );
}

function BarCard({ title, buckets }: { title: string; buckets: Bucket[] }) {
  return (
    <section className="card">
      <h3>{title}</h3>
      {buckets.length === 0 ? (
        <p className="muted">No data.</p>
      ) : (
        <BarRows rows={buckets} format={(n) => String(n)} />
      )}
    </section>
  );
}

function BarRows({ rows, format }: { rows: { label: string; count: number }[]; format: (n: number) => string }) {
  const max = Math.max(...rows.map((b) => b.count), 1);
  return (
    <div className="bar-rows">
      {rows.map((b, i) => (
        <div className="bar-row" key={b.label + i}>
          <div className="bar-label">{b.label}</div>
          <div className="bar-track">
            <div className="bar-fill" style={{ width: `${(b.count / max) * 100}%` }} />
          </div>
          <div className="bar-value">{format(b.count)}</div>
        </div>
      ))}
    </div>
  );
}
