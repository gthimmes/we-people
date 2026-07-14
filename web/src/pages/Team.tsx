import { FormEvent, useEffect, useState } from "react";
import { api, ApiError, ListResponse, Role, UserSummary, Worker } from "../api";
import { useAuth } from "../auth";

export default function Team() {
  const { me } = useAuth();
  const canWrite = !!me?.permissions.includes("user:write");
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [inviting, setInviting] = useState(false);
  const [editingRolesFor, setEditingRolesFor] = useState<string | null>(null);

  async function reload() {
    const [u, r] = await Promise.all([
      api.get<{ data: UserSummary[] }>("/users"),
      api.get<{ data: Role[] }>("/roles"),
    ]);
    setUsers(u.data);
    setRoles(r.data);
  }

  useEffect(() => {
    reload();
    api.get<ListResponse<Worker>>("/workers?limit=200").then((r) => setWorkers(r.data)).catch(() => setWorkers([]));
  }, []);

  const linkedWorkerIds = new Set(users.map((u) => u.worker_id).filter(Boolean));
  const availableWorkers = workers.filter((w) => !linkedWorkerIds.has(w.id));

  async function setStatus(u: UserSummary) {
    const next = u.status === "active" ? "disabled" : "active";
    await api.post(`/users/${u.id}/status`, { status: next });
    reload();
  }

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Team &amp; access</h1>
          <p className="muted">{users.length} users · manage logins and roles</p>
        </div>
        {canWrite && <button className="primary" onClick={() => setInviting((v) => !v)}>{inviting ? "Cancel" : "+ Invite user"}</button>}
      </header>

      {inviting && (
        <InviteForm
          roles={roles}
          availableWorkers={availableWorkers}
          onDone={() => { setInviting(false); reload(); }}
        />
      )}

      <div className="card no-pad">
        <table>
          <thead>
            <tr><th>Email</th><th>Employee</th><th>Roles</th><th>Status</th>{canWrite && <th></th>}</tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td className="mono">{u.email}</td>
                <td className="muted">{u.worker_name ?? "—"}</td>
                <td>
                  {editingRolesFor === u.id ? (
                    <RoleEditor
                      roles={roles}
                      current={u.roles}
                      onSave={async (ids) => { await api.put(`/users/${u.id}/roles`, { role_ids: ids }); setEditingRolesFor(null); reload(); }}
                      onCancel={() => setEditingRolesFor(null)}
                    />
                  ) : (
                    <span>{u.roles.length ? u.roles.join(", ") : <span className="muted">none</span>}</span>
                  )}
                </td>
                <td><span className={`badge badge-${u.status === "active" ? "active" : "terminated"}`}>{u.status}</span></td>
                {canWrite && (
                  <td>
                    <span className="row-actions">
                      <button className="small-btn" onClick={() => setEditingRolesFor(editingRolesFor === u.id ? null : u.id)}>Roles</button>
                      <button className="small-btn" onClick={() => setStatus(u)} disabled={u.id === me?.user_id}>
                        {u.status === "active" ? "Disable" : "Enable"}
                      </button>
                    </span>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <section className="card">
        <h3>Roles</h3>
        <ul className="list">
          {roles.map((r) => (
            <li key={r.id}>
              <span><strong>{r.name}</strong> <span className="muted small">{r.description}</span></span>
              <span className="muted small">{r.permissions.length} permissions</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function InviteForm({ roles, availableWorkers, onDone }: { roles: Role[]; availableWorkers: Worker[]; onDone: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [workerId, setWorkerId] = useState("");
  const [roleIds, setRoleIds] = useState<string[]>([]);
  const [err, setErr] = useState("");

  function toggleRole(id: string) {
    setRoleIds((prev) => (prev.includes(id) ? prev.filter((r) => r !== id) : [...prev, id]));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.post("/users", {
        email,
        password,
        worker_id: workerId || null,
        role_ids: roleIds,
      });
      onDone();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Could not invite user");
    }
  }

  return (
    <form className="card stack" onSubmit={submit}>
      <h3>Invite a user</h3>
      <div className="two-col">
        <label>Email<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label>
        <label>Initial password<input type="text" value={password} onChange={(e) => setPassword(e.target.value)} required placeholder="min 8 chars" /></label>
      </div>
      <label>
        Link to employee (optional)
        <select value={workerId} onChange={(e) => setWorkerId(e.target.value)}>
          <option value="">— standalone login —</option>
          {availableWorkers.map((w) => <option key={w.id} value={w.id}>{w.first_name} {w.last_name} ({w.employee_number})</option>)}
        </select>
      </label>
      <div>
        <div className="field-label">Roles</div>
        <div className="checkbox-list">
          {roles.map((r) => (
            <label key={r.id} className="checkbox-row">
              <input type="checkbox" checked={roleIds.includes(r.id)} onChange={() => toggleRole(r.id)} />
              {r.name}
            </label>
          ))}
        </div>
      </div>
      <p className="muted small">The user signs in with this email + password. (Email invites are coming; for now share the password securely.)</p>
      {err && <div className="error">{err}</div>}
      <button className="primary">Create user</button>
    </form>
  );
}

function RoleEditor({ roles, current, onSave, onCancel }: { roles: Role[]; current: string[]; onSave: (ids: string[]) => Promise<void>; onCancel: () => void }) {
  const [selected, setSelected] = useState<string[]>(roles.filter((r) => current.includes(r.name)).map((r) => r.id));
  function toggle(id: string) {
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  }
  return (
    <div className="stack">
      <div className="checkbox-list">
        {roles.map((r) => (
          <label key={r.id} className="checkbox-row">
            <input type="checkbox" checked={selected.includes(r.id)} onChange={() => toggle(r.id)} />
            {r.name}
          </label>
        ))}
      </div>
      <div className="inline-form">
        <button className="primary small-btn" onClick={() => onSave(selected)}>Save</button>
        <button className="small-btn" onClick={onCancel}>Cancel</button>
      </div>
    </div>
  );
}
