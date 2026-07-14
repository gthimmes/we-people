import { FormEvent, useEffect, useState } from "react";
import { api, ApiError, Department, JobProfile, LegalEntity, Location, Position } from "../api";
import { useAuth } from "../auth";
import CrudPanel, { DeleteButton } from "../components/CrudPanel";

export default function OrgManage() {
  const { me } = useAuth();
  const canWrite = me?.permissions.includes("orgstructure:write");

  const [departments, setDepartments] = useState<Department[]>([]);
  const [locations, setLocations] = useState<Location[]>([]);
  const [entities, setEntities] = useState<LegalEntity[]>([]);
  const [jobProfiles, setJobProfiles] = useState<JobProfile[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);

  async function reload() {
    const [d, l, e, j, p] = await Promise.all([
      api.get<{ data: Department[] }>("/departments"),
      api.get<{ data: Location[] }>("/locations"),
      api.get<{ data: LegalEntity[] }>("/legal-entities"),
      api.get<{ data: JobProfile[] }>("/job-profiles"),
      api.get<{ data: Position[] }>("/positions"),
    ]);
    setDepartments(d.data);
    setLocations(l.data);
    setEntities(e.data);
    setJobProfiles(j.data);
    setPositions(p.data);
  }

  useEffect(() => {
    reload();
  }, []);

  const deptName = (id?: string) => departments.find((d) => d.id === id)?.name ?? "—";
  const locName = (id?: string) => locations.find((l) => l.id === id)?.name ?? "—";

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Organization</h1>
          <p className="muted">Departments, locations, entities, and positions</p>
        </div>
      </header>

      <div className="detail-grid">
        <CrudPanel<Department>
          title="Departments"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "code", label: "Code" },
            { key: "cost_center", label: "Cost center" },
          ]}
          items={departments}
          idOf={(d) => d.id}
          toForm={(d) => ({ name: d.name, code: d.code, cost_center: d.cost_center })}
          summary={(d) => (
            <><strong>{d.name}</strong>{" "}<span className="muted small">{[d.code, d.cost_center].filter(Boolean).join(" · ")}</span></>
          )}
          create={(b) => api.post("/departments", b)}
          update={(id, b) => api.put(`/departments/${id}`, b)}
          remove={(id) => api.del(`/departments/${id}`)}
          onChange={reload}
        />

        <CrudPanel<Location>
          title="Locations"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "city", label: "City" },
            { key: "region", label: "Region" },
            { key: "country", label: "Country" },
            { key: "timezone", label: "Timezone" },
          ]}
          items={locations}
          idOf={(l) => l.id}
          toForm={(l) => ({ name: l.name, city: l.city, region: l.region, country: l.country, timezone: l.timezone })}
          summary={(l) => (
            <><strong>{l.name}</strong>{" "}<span className="muted small">{[l.city, l.region, l.country].filter(Boolean).join(", ")}</span></>
          )}
          create={(b) => api.post("/locations", b)}
          update={(id, b) => api.put(`/locations/${id}`, b)}
          remove={(id) => api.del(`/locations/${id}`)}
          onChange={reload}
        />
      </div>

      <div className="detail-grid">
        <CrudPanel<LegalEntity>
          title="Legal entities"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "country", label: "Country" },
            { key: "tax_id", label: "Tax ID" },
          ]}
          items={entities}
          idOf={(e) => e.id}
          toForm={(e) => ({ name: e.name, country: e.country, tax_id: e.tax_id })}
          summary={(e) => (
            <><strong>{e.name}</strong>{" "}<span className="muted small">{[e.country, e.tax_id].filter(Boolean).join(" · ")}</span></>
          )}
          create={(b) => api.post("/legal-entities", b)}
          update={(id, b) => api.put(`/legal-entities/${id}`, b)}
          remove={(id) => api.del(`/legal-entities/${id}`)}
          onChange={reload}
        />

        <CrudPanel<JobProfile>
          title="Job profiles"
          canWrite={canWrite}
          fields={[
            { key: "title", label: "Title", required: true },
            { key: "job_family", label: "Job family" },
            { key: "level", label: "Level" },
            { key: "flsa_status", label: "FLSA (exempt / non_exempt)" },
          ]}
          items={jobProfiles}
          idOf={(j) => j.id}
          toForm={(j) => ({ title: j.title, job_family: j.job_family, level: j.level, flsa_status: j.flsa_status })}
          summary={(j) => (
            <><strong>{j.title}</strong>{" "}<span className="muted small">{[j.level, j.job_family, j.flsa_status].filter(Boolean).join(" · ")}</span></>
          )}
          create={(b) => api.post("/job-profiles", b)}
          update={(id, b) => api.put(`/job-profiles/${id}`, b)}
          remove={(id) => api.del(`/job-profiles/${id}`)}
          onChange={reload}
        />
      </div>

      <PositionPanel
        positions={positions}
        departments={departments}
        locations={locations}
        jobProfiles={jobProfiles}
        entities={entities}
        canWrite={canWrite}
        deptName={deptName}
        locName={locName}
        onChanged={reload}
      />
    </div>
  );
}

function PositionPanel({
  positions,
  departments,
  locations,
  jobProfiles,
  entities,
  canWrite,
  deptName,
  locName,
  onChanged,
}: {
  positions: Position[];
  departments: Department[];
  locations: Location[];
  jobProfiles: JobProfile[];
  entities: LegalEntity[];
  canWrite?: boolean;
  deptName: (id?: string) => string;
  locName: (id?: string) => string;
  onChanged: () => void;
}) {
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  return (
    <section className="card">
      <div className="panel-head">
        <h3>Positions</h3>
        {canWrite && (
          <button className="small-btn" onClick={() => { setAdding((a) => !a); setEditingId(null); }}>
            {adding ? "Cancel" : "+ Add"}
          </button>
        )}
      </div>
      {adding && (
        <PositionForm
          departments={departments}
          locations={locations}
          jobProfiles={jobProfiles}
          entities={entities}
          onSubmit={async (body) => { await api.post("/positions", body); setAdding(false); onChanged(); }}
          onCancel={() => setAdding(false)}
        />
      )}
      <div className="card no-pad">
        <table>
          <thead>
            <tr><th>Title</th><th>Department</th><th>Location</th><th>Status</th>{canWrite && <th></th>}</tr>
          </thead>
          <tbody>
            {positions.map((p) =>
              editingId === p.id ? (
                <tr key={p.id}>
                  <td colSpan={canWrite ? 5 : 4}>
                    <PositionForm
                      initial={p}
                      departments={departments}
                      locations={locations}
                      jobProfiles={jobProfiles}
                      entities={entities}
                      onSubmit={async (body) => { await api.put(`/positions/${p.id}`, body); setEditingId(null); onChanged(); }}
                      onCancel={() => setEditingId(null)}
                    />
                  </td>
                </tr>
              ) : (
                <tr key={p.id}>
                  <td>{p.title}</td>
                  <td className="muted">{deptName(p.department_id)}</td>
                  <td className="muted">{locName(p.location_id)}</td>
                  <td><span className={`badge badge-${p.status === "filled" ? "active" : "pending"}`}>{p.status}</span></td>
                  {canWrite && (
                    <td>
                      <span className="row-actions">
                        <button className="small-btn" onClick={() => { setEditingId(p.id); setAdding(false); }}>Edit</button>
                        <DeleteButton onDelete={async () => { await api.del(`/positions/${p.id}`); onChanged(); }} />
                      </span>
                    </td>
                  )}
                </tr>
              )
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function PositionForm({
  initial,
  departments,
  locations,
  jobProfiles,
  entities,
  onSubmit,
  onCancel,
}: {
  initial?: Position;
  departments: Department[];
  locations: Location[];
  jobProfiles: JobProfile[];
  entities: LegalEntity[];
  onSubmit: (body: Record<string, unknown>) => Promise<void>;
  onCancel: () => void;
}) {
  const [title, setTitle] = useState(initial?.title ?? "");
  const [departmentId, setDepartmentId] = useState(initial?.department_id ?? "");
  const [locationId, setLocationId] = useState(initial?.location_id ?? "");
  const [jobProfileId, setJobProfileId] = useState(initial?.job_profile_id ?? "");
  const [legalEntityId, setLegalEntityId] = useState(initial?.legal_entity_id ?? "");
  const [status, setStatus] = useState(initial?.status ?? "open");
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await onSubmit({
        title,
        department_id: departmentId || null,
        location_id: locationId || null,
        job_profile_id: jobProfileId || null,
        legal_entity_id: legalEntityId || null,
        status,
      });
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Failed");
    }
  }

  return (
    <form className="stack panel-form" onSubmit={submit}>
      <label>Title<input required value={title} onChange={(e) => setTitle(e.target.value)} /></label>
      <div className="two-col">
        <label>
          Department
          <select value={departmentId} onChange={(e) => setDepartmentId(e.target.value)}>
            <option value="">— none —</option>
            {departments.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </label>
        <label>
          Location
          <select value={locationId} onChange={(e) => setLocationId(e.target.value)}>
            <option value="">— none —</option>
            {locations.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
          </select>
        </label>
      </div>
      <div className="two-col">
        <label>
          Job profile
          <select value={jobProfileId} onChange={(e) => setJobProfileId(e.target.value)}>
            <option value="">— none —</option>
            {jobProfiles.map((j) => <option key={j.id} value={j.id}>{j.title}{j.level ? ` (${j.level})` : ""}</option>)}
          </select>
        </label>
        <label>
          Legal entity
          <select value={legalEntityId} onChange={(e) => setLegalEntityId(e.target.value)}>
            <option value="">— none —</option>
            {entities.map((en) => <option key={en.id} value={en.id}>{en.name}</option>)}
          </select>
        </label>
      </div>
      <label>
        Status
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="open">open</option>
          <option value="filled">filled</option>
          <option value="frozen">frozen</option>
        </select>
      </label>
      {err && <div className="error">{err}</div>}
      <div className="inline-form">
        <button className="primary">Save</button>
        <button type="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  );
}
