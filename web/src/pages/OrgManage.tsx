import { FormEvent, useEffect, useState } from "react";
import { api, ApiError, Department, JobProfile, LegalEntity, Location, Position } from "../api";
import { useAuth } from "../auth";

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
          <p className="muted">Departments, locations, and positions</p>
        </div>
      </header>

      <div className="detail-grid">
        <Panel
          title="Departments"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "code", label: "Code" },
            { key: "cost_center", label: "Cost center" },
          ]}
          create={(body) => api.post("/departments", body)}
          onCreated={reload}
        >
          {departments.map((d) => (
            <li key={d.id}>
              <strong>{d.name}</strong>
              <span className="muted small">{d.code || d.cost_center || ""}</span>
            </li>
          ))}
        </Panel>

        <Panel
          title="Locations"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "city", label: "City" },
            { key: "region", label: "Region" },
            { key: "country", label: "Country" },
          ]}
          create={(body) => api.post("/locations", body)}
          onCreated={reload}
        >
          {locations.map((l) => (
            <li key={l.id}>
              <strong>{l.name}</strong>
              <span className="muted small">{[l.city, l.region, l.country].filter(Boolean).join(", ")}</span>
            </li>
          ))}
        </Panel>
      </div>

      <div className="detail-grid">
        <Panel
          title="Legal entities"
          canWrite={canWrite}
          fields={[
            { key: "name", label: "Name", required: true },
            { key: "country", label: "Country" },
            { key: "tax_id", label: "Tax ID" },
          ]}
          create={(body) => api.post("/legal-entities", body)}
          onCreated={reload}
        >
          {entities.map((e) => (
            <li key={e.id}>
              <strong>{e.name}</strong>
              <span className="muted small">{[e.country, e.tax_id].filter(Boolean).join(" · ")}</span>
            </li>
          ))}
        </Panel>

        <Panel
          title="Job profiles"
          canWrite={canWrite}
          fields={[
            { key: "title", label: "Title", required: true },
            { key: "job_family", label: "Job family" },
            { key: "level", label: "Level" },
          ]}
          create={(body) => api.post("/job-profiles", body)}
          onCreated={reload}
        >
          {jobProfiles.map((j) => (
            <li key={j.id}>
              <strong>{j.title}</strong>
              <span className="muted small">{[j.level, j.job_family].filter(Boolean).join(" · ")}</span>
            </li>
          ))}
        </Panel>
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
        onCreated={reload}
      />
    </div>
  );
}

interface FieldDef {
  key: string;
  label: string;
  required?: boolean;
}

function Panel({
  title,
  fields,
  create,
  onCreated,
  canWrite,
  children,
}: {
  title: string;
  fields: FieldDef[];
  create: (body: Record<string, string>) => Promise<unknown>;
  onCreated: () => void;
  canWrite?: boolean;
  children: React.ReactNode;
}) {
  const [show, setShow] = useState(false);
  const [form, setForm] = useState<Record<string, string>>({});
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await create(form);
      setForm({});
      setShow(false);
      onCreated();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Failed");
    }
  }

  return (
    <section className="card">
      <div className="panel-head">
        <h3>{title}</h3>
        {canWrite && (
          <button className="small-btn" onClick={() => setShow((s) => !s)}>
            {show ? "Cancel" : "+ Add"}
          </button>
        )}
      </div>
      {show && (
        <form className="stack panel-form" onSubmit={submit}>
          {fields.map((f) => (
            <label key={f.key}>
              {f.label}
              <input
                required={f.required}
                value={form[f.key] ?? ""}
                onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
              />
            </label>
          ))}
          {err && <div className="error">{err}</div>}
          <button className="primary">Save</button>
        </form>
      )}
      <ul className="list">{children}</ul>
    </section>
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
  onCreated,
}: {
  positions: Position[];
  departments: Department[];
  locations: Location[];
  jobProfiles: JobProfile[];
  entities: LegalEntity[];
  canWrite?: boolean;
  deptName: (id?: string) => string;
  locName: (id?: string) => string;
  onCreated: () => void;
}) {
  const [show, setShow] = useState(false);
  const [title, setTitle] = useState("");
  const [departmentId, setDepartmentId] = useState("");
  const [locationId, setLocationId] = useState("");
  const [jobProfileId, setJobProfileId] = useState("");
  const [legalEntityId, setLegalEntityId] = useState("");
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.post("/positions", {
        title,
        department_id: departmentId || null,
        location_id: locationId || null,
        job_profile_id: jobProfileId || null,
        legal_entity_id: legalEntityId || null,
      });
      setTitle("");
      setDepartmentId("");
      setLocationId("");
      setJobProfileId("");
      setLegalEntityId("");
      setShow(false);
      onCreated();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Failed");
    }
  }

  return (
    <section className="card">
      <div className="panel-head">
        <h3>Positions</h3>
        {canWrite && (
          <button className="small-btn" onClick={() => setShow((s) => !s)}>
            {show ? "Cancel" : "+ Add"}
          </button>
        )}
      </div>
      {show && (
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
          {err && <div className="error">{err}</div>}
          <button className="primary">Save</button>
        </form>
      )}
      <div className="card no-pad">
        <table>
          <thead>
            <tr><th>Title</th><th>Department</th><th>Location</th><th>Status</th></tr>
          </thead>
          <tbody>
            {positions.map((p) => (
              <tr key={p.id}>
                <td>{p.title}</td>
                <td className="muted">{deptName(p.department_id)}</td>
                <td className="muted">{locName(p.location_id)}</td>
                <td><span className={`badge badge-${p.status === "filled" ? "active" : "pending"}`}>{p.status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
