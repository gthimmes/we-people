import { FormEvent, useEffect, useState } from "react";
import {
  api,
  ApiError,
  ChecklistPlan,
  ChecklistTemplate,
  ChecklistTemplateTask,
  ListResponse,
  PlanWithTasks,
  Worker,
} from "../api";
import { useAuth } from "../auth";
import { DeleteButton } from "../components/CrudPanel";

export default function Onboarding() {
  const { me } = useAuth();
  const canWrite = !!me?.permissions.includes("worker:write");
  const [plans, setPlans] = useState<ChecklistPlan[]>([]);
  const [templates, setTemplates] = useState<ChecklistTemplate[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [starting, setStarting] = useState(false);
  const [addingTemplate, setAddingTemplate] = useState(false);
  const [openPlan, setOpenPlan] = useState<string | null>(null);

  async function reload() {
    const [p, t] = await Promise.all([
      api.get<{ data: ChecklistPlan[] }>("/checklist-plans"),
      api.get<{ data: ChecklistTemplate[] }>("/checklist-templates"),
    ]);
    setPlans(p.data);
    setTemplates(t.data);
  }

  useEffect(() => {
    reload();
    api.get<ListResponse<Worker>>("/workers?limit=200").then((r) => setWorkers(r.data)).catch(() => setWorkers([]));
  }, []);

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Onboarding</h1>
          <p className="muted">Checklists for new hires and departures</p>
        </div>
        {canWrite && <button className="primary" onClick={() => setStarting((s) => !s)}>{starting ? "Cancel" : "Start a checklist"}</button>}
      </header>

      {starting && (
        <StartForm
          workers={workers}
          templates={templates}
          onDone={() => { setStarting(false); reload(); }}
        />
      )}

      <section className="card">
        <h3>Active checklists</h3>
        {plans.length === 0 ? (
          <p className="muted">No checklists yet.</p>
        ) : (
          <ul className="list">
            {plans.map((p) => (
              <li key={p.id} className="plan-row">
                <div className="plan-main" onClick={() => setOpenPlan(openPlan === p.id ? null : p.id)}>
                  <div>
                    <strong>{p.worker_name}</strong>
                    <span className="muted small"> · {p.name} · <span className={`badge badge-${p.type === "offboarding" ? "pending" : "active"}`}>{p.type}</span></span>
                  </div>
                  <Progress done={p.done_tasks} total={p.total_tasks} />
                </div>
                {openPlan === p.id && <PlanTasks planId={p.id} canOverride={canWrite} onChange={reload} />}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="card">
        <div className="panel-head">
          <h3>Templates</h3>
          {canWrite && <button className="small-btn" onClick={() => setAddingTemplate((a) => !a)}>{addingTemplate ? "Cancel" : "+ New template"}</button>}
        </div>
        {addingTemplate && <TemplateForm onDone={() => { setAddingTemplate(false); reload(); }} />}
        <ul className="list">
          {templates.map((t) => (
            <li key={t.id}>
              <span>
                <strong>{t.name}</strong>
                <span className="muted small"> · {t.type} · {t.tasks.length} tasks</span>
              </span>
              {canWrite && <DeleteButton onDelete={async () => { await api.del(`/checklist-templates/${t.id}`); reload(); }} />}
            </li>
          ))}
          {templates.length === 0 && <li className="muted">No templates yet.</li>}
        </ul>
      </section>
    </div>
  );
}

export function Progress({ done, total }: { done: number; total: number }) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return (
    <div className="progress-wrap" title={`${done}/${total} done`}>
      <div className="progress-bar"><div className="progress-fill" style={{ width: `${pct}%` }} /></div>
      <span className="muted small">{done}/{total}</span>
    </div>
  );
}

export function PlanTasks({ planId, canOverride, onChange }: { planId: string; canOverride: boolean; onChange: () => void }) {
  const [plan, setPlan] = useState<PlanWithTasks | null>(null);
  const [err, setErr] = useState("");

  async function load() {
    setPlan(await api.get<PlanWithTasks>(`/checklist-plans/${planId}`));
  }
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [planId]);

  async function toggle(taskId: string, done: boolean) {
    setErr("");
    try {
      await api.post(`/checklist-tasks/${taskId}/status`, { status: done ? "done" : "pending" });
      await load();
      onChange();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Could not update task");
    }
  }

  if (!plan) return <div className="muted small plan-tasks">Loading…</div>;
  return (
    <div className="plan-tasks">
      {err && <div className="error small">{err}</div>}
      <ul className="task-list">
        {plan.tasks.map((t) => (
          <li key={t.id}>
            <label className="task-check">
              <input type="checkbox" checked={t.status === "done"} onChange={(e) => toggle(t.id, e.target.checked)} disabled={!canOverride && !t.assignee_name} />
              <span className={t.status === "done" ? "task-done" : ""}>{t.title}</span>
            </label>
            <span className="muted small">
              {t.assignee_name ?? "HR"}{t.due_date ? ` · due ${t.due_date.slice(0, 10)}` : ""}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function StartForm({ workers, templates, onDone }: { workers: Worker[]; templates: ChecklistTemplate[]; onDone: () => void }) {
  const [workerId, setWorkerId] = useState("");
  const [templateId, setTemplateId] = useState(templates[0]?.id ?? "");
  const [startDate, setStartDate] = useState("");
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.post("/checklist-plans", { worker_id: workerId, template_id: templateId, start_date: startDate || null });
      onDone();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Could not start checklist");
    }
  }

  return (
    <form className="card stack" onSubmit={submit}>
      <h3>Start a checklist</h3>
      <div className="two-col">
        <label>
          Employee
          <select value={workerId} onChange={(e) => setWorkerId(e.target.value)} required>
            <option value="">Select…</option>
            {workers.map((w) => <option key={w.id} value={w.id}>{w.first_name} {w.last_name}</option>)}
          </select>
        </label>
        <label>
          Template
          <select value={templateId} onChange={(e) => setTemplateId(e.target.value)} required>
            {templates.map((t) => <option key={t.id} value={t.id}>{t.name} ({t.type})</option>)}
          </select>
        </label>
      </div>
      <label>Start date<input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} /></label>
      {err && <div className="error">{err}</div>}
      <button className="primary">Start</button>
    </form>
  );
}

const blankTask: ChecklistTemplateTask = { title: "", description: "", assignee: "new_hire", offset_days: 0 };

function TemplateForm({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState("");
  const [type, setType] = useState("onboarding");
  const [tasks, setTasks] = useState<ChecklistTemplateTask[]>([{ ...blankTask }]);
  const [err, setErr] = useState("");

  function setTask(i: number, patch: Partial<ChecklistTemplateTask>) {
    setTasks((prev) => prev.map((t, idx) => (idx === i ? { ...t, ...patch } : t)));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.post("/checklist-templates", { name, type, tasks: tasks.filter((t) => t.title.trim()) });
      onDone();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Could not create template");
    }
  }

  return (
    <form className="card stack" onSubmit={submit}>
      <div className="two-col">
        <label>Template name<input value={name} onChange={(e) => setName(e.target.value)} required /></label>
        <label>
          Type
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="onboarding">onboarding</option>
            <option value="offboarding">offboarding</option>
          </select>
        </label>
      </div>
      <div className="field-label">Tasks</div>
      {tasks.map((t, i) => (
        <div key={i} className="template-task-row">
          <input placeholder="Task title" value={t.title} onChange={(e) => setTask(i, { title: e.target.value })} />
          <select value={t.assignee} onChange={(e) => setTask(i, { assignee: e.target.value })}>
            <option value="new_hire">New hire</option>
            <option value="manager">Manager</option>
            <option value="hr">HR</option>
          </select>
          <input type="number" title="Days after start" value={t.offset_days} onChange={(e) => setTask(i, { offset_days: Number(e.target.value) })} className="offset-input" />
          <button type="button" className="small-btn" onClick={() => setTasks((prev) => prev.filter((_, idx) => idx !== i))}>✕</button>
        </div>
      ))}
      <button type="button" className="small-btn" onClick={() => setTasks((prev) => [...prev, { ...blankTask }])}>+ Add task</button>
      {err && <div className="error">{err}</div>}
      <button className="primary">Create template</button>
    </form>
  );
}
