import { FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError, ListResponse, Worker } from "../api";
import { useAuth } from "../auth";

const empty = {
  employee_number: "",
  first_name: "",
  last_name: "",
  work_email: "",
  hire_date: "",
};

export default function Directory() {
  const { me } = useAuth();
  const navigate = useNavigate();
  const canWrite = me?.permissions.includes("worker:write");

  const [workers, setWorkers] = useState<Worker[]>([]);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState(empty);
  const [error, setError] = useState("");

  async function load(q = search) {
    setLoading(true);
    try {
      const res = await api.get<ListResponse<Worker>>(
        `/workers?search=${encodeURIComponent(q)}&limit=100`
      );
      setWorkers(res.data);
      setTotal(res.meta.total);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function create(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await api.post<Worker>("/workers", { ...form, hire_date: form.hire_date || null });
      setForm(empty);
      setShowForm(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create person");
    }
  }

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>People</h1>
          <p className="muted">{total} in your organization</p>
        </div>
        {canWrite && (
          <button className="primary" onClick={() => setShowForm((s) => !s)}>
            {showForm ? "Cancel" : "+ Add person"}
          </button>
        )}
      </header>

      {showForm && (
        <form className="card form-grid" onSubmit={create}>
          <label>
            Employee #
            <input
              value={form.employee_number}
              onChange={(e) => setForm({ ...form, employee_number: e.target.value })}
              required
            />
          </label>
          <label>
            First name
            <input value={form.first_name} onChange={(e) => setForm({ ...form, first_name: e.target.value })} required />
          </label>
          <label>
            Last name
            <input value={form.last_name} onChange={(e) => setForm({ ...form, last_name: e.target.value })} required />
          </label>
          <label>
            Work email
            <input type="email" value={form.work_email} onChange={(e) => setForm({ ...form, work_email: e.target.value })} />
          </label>
          <label>
            Hire date
            <input type="date" value={form.hire_date} onChange={(e) => setForm({ ...form, hire_date: e.target.value })} />
          </label>
          <div className="form-actions">
            {error && <span className="error">{error}</span>}
            <button className="primary">Save</button>
          </div>
        </form>
      )}

      <div className="toolbar">
        <input
          className="search"
          placeholder="Search by name, email, or employee #"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && load()}
        />
        <button onClick={() => load()}>Search</button>
      </div>

      <div className="card no-pad">
        <table>
          <thead>
            <tr>
              <th>Employee #</th>
              <th>Name</th>
              <th>Work email</th>
              <th>Hire date</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5} className="muted center-cell">Loading…</td>
              </tr>
            ) : workers.length === 0 ? (
              <tr>
                <td colSpan={5} className="muted center-cell">No people found.</td>
              </tr>
            ) : (
              workers.map((w) => (
                <tr key={w.id} className="clickable" onClick={() => navigate(`/people/${w.id}`)}>
                  <td className="mono">{w.employee_number}</td>
                  <td>
                    {w.first_name} {w.last_name}
                  </td>
                  <td className="muted">{w.work_email || "—"}</td>
                  <td className="muted">{w.hire_date?.slice(0, 10) ?? "—"}</td>
                  <td>
                    <span className={`badge badge-${w.status}`}>{w.status}</span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
