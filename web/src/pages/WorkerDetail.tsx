import { FormEvent, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  api,
  ApiError,
  Assignment,
  ChecklistPlan,
  Document as Doc,
  EmergencyContact,
  LifecycleEvent,
  ListResponse,
  Position,
  Profile,
  Worker,
} from "../api";
import { useAuth } from "../auth";
import { DeleteButton } from "../components/CrudPanel";
import { PlanTasks, Progress } from "./Onboarding";

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
  const navigate = useNavigate();
  const { me } = useAuth();
  const canWrite = me?.permissions.includes("worker:write");
  const canAssign = me?.permissions.includes("orgstructure:write");

  const [profile, setProfile] = useState<Profile | null>(null);
  const [events, setEvents] = useState<LifecycleEvent[]>([]);
  const [contacts, setContacts] = useState<EmergencyContact[]>([]);
  const [docs, setDocs] = useState<Doc[]>([]);
  const [plans, setPlans] = useState<ChecklistPlan[]>([]);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState(false);

  async function reload() {
    const [p, ev, cs, ds, pl] = await Promise.all([
      api.get<Profile>(`/workers/${id}/profile`),
      api.get<{ data: LifecycleEvent[] }>(`/workers/${id}/events`),
      api.get<{ data: EmergencyContact[] }>(`/workers/${id}/emergency-contacts`),
      api.get<{ data: Doc[] }>(`/documents?worker_id=${id}`),
      api.get<{ data: ChecklistPlan[] }>(`/checklist-plans?worker_id=${id}`),
    ]);
    setProfile(p);
    setEvents(ev.data);
    setContacts(cs.data);
    setDocs(ds.data);
    setPlans(pl.data);
  }

  useEffect(() => {
    reload().catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (error) return <div className="error">{error}</div>;
  if (!profile) return <div className="muted">Loading…</div>;

  const address = [profile.address_line1, profile.address_line2, profile.city, profile.region, profile.postal_code, profile.country]
    .filter(Boolean)
    .join(", ");

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
          {canWrite && (
            <>
              {profile.status !== "terminated" && (
                <>
                  <button onClick={() => setEditing((e) => !e)}>{editing ? "Cancel" : "Edit"}</button>
                  <TerminateButton id={id} onDone={reload} />
                </>
              )}
              <DeleteButton
                label="Delete"
                onDelete={async () => { await api.del(`/workers/${id}`); navigate("/directory"); }}
              />
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
              <Field label="Address" value={address} />
              <Field label="Gender" value={profile.gender} />
              <Field label="Ethnicity" value={profile.ethnicity} />
              <Field label="Marital status" value={profile.marital_status} />
              <Field label="Work authorization" value={profile.work_auth_type} />
              <Field
                label="I-9 verified"
                value={profile.i9_verified ? `Yes${profile.i9_verified_on ? " · " + profile.i9_verified_on.slice(0, 10) : ""}` : "No"}
              />
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

      <div className="detail-grid">
        <ContactsSection
          workerId={id}
          contacts={contacts}
          canWrite={!!canWrite && profile.status !== "terminated"}
          onChange={reload}
        />
        <DocumentsSection
          workerId={id}
          docs={docs}
          canWrite={!!canWrite}
          onChange={reload}
        />
      </div>

      {plans.length > 0 && (
        <section className="card">
          <h3>Checklists</h3>
          <ul className="list">
            {plans.map((pl) => (
              <li key={pl.id} className="plan-row">
                <div className="plan-main">
                  <span><strong>{pl.name}</strong> <span className={`badge badge-${pl.type === "offboarding" ? "pending" : "active"}`}>{pl.type}</span></span>
                  <Progress done={pl.done_tasks} total={pl.total_tasks} />
                </div>
                <PlanTasks planId={pl.id} canOverride={!!canWrite} onChange={reload} />
              </li>
            ))}
          </ul>
        </section>
      )}

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
    address_line1: profile.address_line1,
    address_line2: profile.address_line2,
    city: profile.city,
    region: profile.region,
    postal_code: profile.postal_code,
    country: profile.country,
    gender: profile.gender,
    ethnicity: profile.ethnicity,
    marital_status: profile.marital_status,
    work_auth_type: profile.work_auth_type,
    work_auth_expiry: profile.work_auth_expiry?.slice(0, 10) ?? "",
    i9_verified_on: profile.i9_verified_on?.slice(0, 10) ?? "",
  });
  const [i9Verified, setI9Verified] = useState(profile.i9_verified);
  const [err, setErr] = useState("");
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });

  async function save(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.put<Worker>(`/workers/${profile.id}`, {
        ...f,
        employee_number: profile.employee_number,
        hire_date: f.hire_date || null,
        work_auth_expiry: f.work_auth_expiry || null,
        i9_verified: i9Verified,
        i9_verified_on: f.i9_verified_on || null,
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
        <label>First name<input value={f.first_name} onChange={set("first_name")} /></label>
        <label>Last name<input value={f.last_name} onChange={set("last_name")} /></label>
      </div>
      <label>Work email<input value={f.work_email} onChange={set("work_email")} /></label>
      <label>Personal email<input value={f.personal_email} onChange={set("personal_email")} /></label>
      <div className="two-col">
        <label>Phone<input value={f.phone} onChange={set("phone")} /></label>
        <label>Hire date<input type="date" value={f.hire_date} onChange={set("hire_date")} /></label>
      </div>
      <label>Address<input value={f.address_line1} onChange={set("address_line1")} placeholder="Street" /></label>
      <div className="two-col">
        <label>City<input value={f.city} onChange={set("city")} /></label>
        <label>Region/State<input value={f.region} onChange={set("region")} /></label>
      </div>
      <div className="two-col">
        <label>Postal code<input value={f.postal_code} onChange={set("postal_code")} /></label>
        <label>Country<input value={f.country} onChange={set("country")} /></label>
      </div>
      <div className="two-col">
        <label>Gender<input value={f.gender} onChange={set("gender")} /></label>
        <label>Marital status<input value={f.marital_status} onChange={set("marital_status")} /></label>
      </div>
      <label>Ethnicity<input value={f.ethnicity} onChange={set("ethnicity")} /></label>
      <div className="two-col">
        <label>Work authorization<input value={f.work_auth_type} onChange={set("work_auth_type")} placeholder="citizen, visa, …" /></label>
        <label>Work auth expiry<input type="date" value={f.work_auth_expiry} onChange={set("work_auth_expiry")} /></label>
      </div>
      <div className="two-col">
        <label className="checkbox-row">
          <input type="checkbox" checked={i9Verified} onChange={(e) => setI9Verified(e.target.checked)} />
          I-9 verified
        </label>
        <label>I-9 verified on<input type="date" value={f.i9_verified_on} onChange={set("i9_verified_on")} /></label>
      </div>
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
  const [eventType, setEventType] = useState("transfer");
  const [reason, setReason] = useState("");
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
      event_type: eventType,
      reason,
    });
    setOpen(false);
    onAssigned();
  }

  if (!open) return <button className="mt" onClick={() => setOpen(true)}>Change assignment</button>;
  return (
    <form className="stack mt" onSubmit={submit}>
      <label>
        Change type
        <select value={eventType} onChange={(e) => setEventType(e.target.value)}>
          <option value="transfer">Transfer</option>
          <option value="promotion">Promotion</option>
        </select>
      </label>
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
      <label>Reason<input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="e.g. reorg, promotion" /></label>
      <div className="inline-form">
        <button className="primary">Save assignment</button>
        <button type="button" onClick={() => setOpen(false)}>Cancel</button>
      </div>
    </form>
  );
}

function ContactsSection({
  workerId,
  contacts,
  canWrite,
  onChange,
}: {
  workerId: string;
  contacts: EmergencyContact[];
  canWrite: boolean;
  onChange: () => void;
}) {
  const [show, setShow] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  return (
    <section className="card">
      <div className="panel-head">
        <h3>Emergency contacts</h3>
        {canWrite && <button className="small-btn" onClick={() => { setShow((s) => !s); setEditingId(null); }}>{show ? "Cancel" : "+ Add"}</button>}
      </div>
      {show && (
        <ContactForm
          onSubmit={async (body) => { await api.post(`/workers/${workerId}/emergency-contacts`, body); setShow(false); onChange(); }}
          onCancel={() => setShow(false)}
        />
      )}
      {contacts.length === 0 ? (
        <p className="muted">No emergency contacts.</p>
      ) : (
        <ul className="list">
          {contacts.map((c) =>
            editingId === c.id ? (
              <li key={c.id} className="crud-editing">
                <ContactForm
                  initial={c}
                  onSubmit={async (body) => { await api.put(`/workers/${workerId}/emergency-contacts/${c.id}`, body); setEditingId(null); onChange(); }}
                  onCancel={() => setEditingId(null)}
                />
              </li>
            ) : (
              <li key={c.id}>
                <span>
                  <strong>{c.name}</strong>
                  {c.relationship && <span className="muted"> · {c.relationship}</span>}
                  <div className="muted small">{[c.phone, c.email].filter(Boolean).join(" · ")}</div>
                </span>
                {canWrite && (
                  <span className="row-actions">
                    <button className="small-btn" onClick={() => { setEditingId(c.id); setShow(false); }}>Edit</button>
                    <DeleteButton label="Remove" onDelete={async () => { await api.del(`/workers/${workerId}/emergency-contacts/${c.id}`); onChange(); }} />
                  </span>
                )}
              </li>
            )
          )}
        </ul>
      )}
    </section>
  );
}

function ContactForm({ initial, onSubmit, onCancel }: { initial?: EmergencyContact; onSubmit: (b: Record<string, string>) => Promise<void>; onCancel: () => void }) {
  const [f, setF] = useState({
    name: initial?.name ?? "",
    relationship: initial?.relationship ?? "",
    phone: initial?.phone ?? "",
    email: initial?.email ?? "",
  });
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await onSubmit(f);
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Failed");
    }
  }

  return (
    <form className="stack panel-form" onSubmit={submit}>
      <div className="two-col">
        <label>Name<input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} required /></label>
        <label>Relationship<input value={f.relationship} onChange={(e) => setF({ ...f, relationship: e.target.value })} /></label>
      </div>
      <div className="two-col">
        <label>Phone<input value={f.phone} onChange={(e) => setF({ ...f, phone: e.target.value })} /></label>
        <label>Email<input value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} /></label>
      </div>
      {err && <div className="error">{err}</div>}
      <div className="inline-form">
        <button className="primary">Save contact</button>
        <button type="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  );
}

function DocumentsSection({
  workerId,
  docs,
  canWrite,
  onChange,
}: {
  workerId: string;
  docs: Doc[];
  canWrite: boolean;
  onChange: () => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function upload(e: FormEvent) {
    e.preventDefault();
    const file = fileRef.current?.files?.[0];
    if (!file) return;
    setBusy(true);
    setErr("");
    try {
      const form = new FormData();
      form.append("file", file);
      form.append("worker_id", workerId);
      form.append("name", file.name);
      await api.upload("/documents", form);
      if (fileRef.current) fileRef.current.value = "";
      onChange();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Upload failed");
    } finally {
      setBusy(false);
    }
  }
  async function remove(did: string) {
    await api.del(`/documents/${did}`);
    onChange();
  }

  return (
    <section className="card">
      <div className="panel-head">
        <h3>Documents</h3>
      </div>
      {canWrite && (
        <form className="inline-form panel-form" onSubmit={upload}>
          <input ref={fileRef} type="file" />
          <button className="primary" disabled={busy}>{busy ? "Uploading…" : "Upload"}</button>
        </form>
      )}
      {err && <div className="error">{err}</div>}
      {docs.length === 0 ? (
        <p className="muted">No documents.</p>
      ) : (
        <ul className="list">
          {docs.map((d) => (
            <li key={d.id}>
              <span>
                <button className="link" onClick={() => api.download(`/documents/${d.id}/download`, d.name)}>
                  {d.name}
                </button>
                <div className="muted small">{(d.size_bytes / 1024).toFixed(1)} KB · {d.created_at.slice(0, 10)}</div>
              </span>
              {canWrite && <button className="small-btn danger" onClick={() => remove(d.id)}>Delete</button>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
