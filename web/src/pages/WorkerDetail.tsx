import { FormEvent, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  api,
  ApiError,
  Assignment,
  LifecycleEvent,
  ListResponse,
  Position,
  Profile,
  Worker,
} from "../api";
import { useAuth } from "../auth";

const eventLabels: Record<string, string> = {
  hire: "Hired",
  transfer: "Transferred",
  promotion: "Promoted",
  leave: "Went on leave",
  termination: "Terminated",
  rehire: "Rehired",
};

export default function WorkerDetail() {
  const { id = "" } = useParams();
  const { me } = useAuth();
  const canWrite = me?.permissions.includes("worker:write");
  const canAssign = me?.permissions.includes("orgstructure:write");

  const [profile, setProfile] = useState<Profile | null>(null);
  const [events, setEvents] = useState<LifecycleEvent[]>([]);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState(false);

  async function reload() {
    const [p, ev] = await Promise.all([
      api.get<Profile>(`/workers/${id}/profile`),
      api.get<{ data: LifecycleEvent[] }>(`/workers/${id}/events`),
    ]);
    setProfile(p);
    setEvents(ev.data);
  }

  useEffect(() => {
    reload().catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (error) return <div className="error">{error}</div>;
  if (!profile) return <div className="muted">Loading…</div>;

  return (
    <div>
      <Link to="/directory" className="back-link">
        ← People
      </Link>
      <header className="page-header">
        <div>
          <h1>
            {profile.first_name} {profile.last_name}{" "}
            <span className={`badge badge-${profile.status}`}>{profile.status}</span>
          </h1>
          <p className="muted mono">{profile.employee_number}</p>
        </div>
        <div className="header-actions">
          {canWrite && profile.status !== "terminated" && (
            <>
              <button onClick={() => setEditing((e) => !e)}>{editing ? "Cancel" : "Edit"}</button>
              <TerminateButton id={id} onDone={reload} />
            </>
          )}
        </div>
      </header>

      <div className="detail-grid">
        <section className="card">
          <h3>Profile</h3>
          {editing ? (
            <EditForm profile={profile} onSaved={() => { setEditing(false); reload(); }} />
          ) : (
            <dl className="fields">
              <Field label="Work email" value={profile.work_email} />
              <Field label="Personal email" value={profile.personal_email} />
              <Field label="Phone" value={profile.phone} />
              <Field label="Hire date" value={profile.hire_date?.slice(0, 10)} />
              <Field label="Date of birth" value={profile.date_of_birth?.slice(0, 10)} />
            </dl>
          )}
        </section>

        <section className="card">
          <h3>Current role</h3>
          <dl className="fields">
            <Field label="Position" value={profile.position_title} />
            <Field label="Department" value={profile.department_name} />
            <Field label="Location" value={profile.location_name} />
            <Field label="Manager" value={profile.manager_name} />
          </dl>
          {canAssign && profile.status !== "terminated" && (
            <AssignForm workerId={id} onAssigned={reload} />
          )}
        </section>
      </div>

      <section className="card">
        <h3>Employment history</h3>
        <ol className="timeline">
          {events.map((e) => (
            <li key={e.id}>
              <div className="timeline-dot" />
              <div>
                <strong>{eventLabels[e.type] ?? e.type}</strong>
                <span className="muted"> · {e.effective_date.slice(0, 10)}</span>
                {e.reason && <div className="muted small">{e.reason}</div>}
              </div>
            </li>
          ))}
        </ol>
      </section>
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <>
      <dt>{label}</dt>
      <dd className={value ? "" : "muted"}>{value || "—"}</dd>
    </>
  );
}

function EditForm({ profile, onSaved }: { profile: Profile; onSaved: () => void }) {
  const [f, setF] = useState({
    first_name: profile.first_name,
    last_name: profile.last_name,
    work_email: profile.work_email,
    personal_email: profile.personal_email,
    phone: profile.phone,
    hire_date: profile.hire_date?.slice(0, 10) ?? "",
  });
  const [err, setErr] = useState("");

  async function save(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.put<Worker>(`/workers/${profile.id}`, {
        ...f,
        employee_number: profile.employee_number,
        hire_date: f.hire_date || null,
        status: profile.status,
      });
      onSaved();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Save failed");
    }
  }

  return (
    <form className="stack" onSubmit={save}>
      <div className="two-col">
        <label>First name<input value={f.first_name} onChange={(e) => setF({ ...f, first_name: e.target.value })} /></label>
        <label>Last name<input value={f.last_name} onChange={(e) => setF({ ...f, last_name: e.target.value })} /></label>
      </div>
      <label>Work email<input value={f.work_email} onChange={(e) => setF({ ...f, work_email: e.target.value })} /></label>
      <label>Personal email<input value={f.personal_email} onChange={(e) => setF({ ...f, personal_email: e.target.value })} /></label>
      <label>Phone<input value={f.phone} onChange={(e) => setF({ ...f, phone: e.target.value })} /></label>
      <label>Hire date<input type="date" value={f.hire_date} onChange={(e) => setF({ ...f, hire_date: e.target.value })} /></label>
      {err && <div className="error">{err}</div>}
      <button className="primary">Save changes</button>
    </form>
  );
}

function TerminateButton({ id, onDone }: { id: string; onDone: () => void }) {
  const [open, setOpen] = useState(false);
  const [date, setDate] = useState("");
  const [reason, setReason] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    await api.post(`/workers/${id}/terminate`, { effective_date: date || null, reason });
    setOpen(false);
    onDone();
  }

  if (!open) return <button className="danger" onClick={() => setOpen(true)}>Terminate</button>;
  return (
    <form className="inline-form" onSubmit={submit}>
      <input type="date" value={date} onChange={(e) => setDate(e.target.value)} title="Effective date" />
      <input placeholder="Reason" value={reason} onChange={(e) => setReason(e.target.value)} />
      <button className="danger">Confirm</button>
      <button type="button" onClick={() => setOpen(false)}>Cancel</button>
    </form>
  );
}

function AssignForm({ workerId, onAssigned }: { workerId: string; onAssigned: () => void }) {
  const [positions, setPositions] = useState<Position[]>([]);
  const [managers, setManagers] = useState<Worker[]>([]);
  const [positionId, setPositionId] = useState("");
  const [managerId, setManagerId] = useState("");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    api.get<{ data: Position[] }>("/positions").then((r) => setPositions(r.data));
    api.get<ListResponse<Worker>>("/workers?limit=200").then((r) => setManagers(r.data.filter((w) => w.id !== workerId)));
  }, [open, workerId]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    await api.post<Assignment>("/assignments", {
      worker_id: workerId,
      position_id: positionId || null,
      manager_id: managerId || null,
    });
    setOpen(false);
    onAssigned();
  }

  if (!open) return <button className="mt" onClick={() => setOpen(true)}>Change assignment</button>;
  return (
    <form className="stack mt" onSubmit={submit}>
      <label>
        Position
        <select value={positionId} onChange={(e) => setPositionId(e.target.value)}>
          <option value="">— none —</option>
          {positions.map((p) => (
            <option key={p.id} value={p.id}>{p.title}</option>
          ))}
        </select>
      </label>
      <label>
        Manager
        <select value={managerId} onChange={(e) => setManagerId(e.target.value)}>
          <option value="">— none —</option>
          {managers.map((m) => (
            <option key={m.id} value={m.id}>{m.first_name} {m.last_name}</option>
          ))}
        </select>
      </label>
      <div className="inline-form">
        <button className="primary">Save assignment</button>
        <button type="button" onClick={() => setOpen(false)}>Cancel</button>
      </div>
    </form>
  );
}
