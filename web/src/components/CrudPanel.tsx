import { FormEvent, ReactNode, useState } from "react";
import { ApiError } from "../api";

export interface CrudField {
  key: string;
  label: string;
  required?: boolean;
}

interface CrudPanelProps<T> {
  title: string;
  canWrite?: boolean;
  fields: CrudField[];
  items: T[];
  idOf: (item: T) => string;
  toForm: (item: T) => Record<string, string>;
  summary: (item: T) => ReactNode;
  create: (body: Record<string, string>) => Promise<unknown>;
  update: (id: string, body: Record<string, string>) => Promise<unknown>;
  remove: (id: string) => Promise<unknown>;
  onChange: () => void;
}

// CrudPanel renders a titled list of records with inline add, edit, and
// delete-with-confirm — no blocking browser dialogs.
export default function CrudPanel<T>({
  title,
  canWrite,
  fields,
  items,
  idOf,
  toForm,
  summary,
  create,
  update,
  remove,
  onChange,
}: CrudPanelProps<T>) {
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  return (
    <section className="card">
      <div className="panel-head">
        <h3>{title}</h3>
        {canWrite && (
          <button className="small-btn" onClick={() => { setAdding((a) => !a); setEditingId(null); }}>
            {adding ? "Cancel" : "+ Add"}
          </button>
        )}
      </div>

      {adding && (
        <FieldForm
          fields={fields}
          initial={{}}
          submitLabel="Save"
          onSubmit={async (body) => { await create(body); setAdding(false); onChange(); }}
          onCancel={() => setAdding(false)}
        />
      )}

      <ul className="list">
        {items.map((item) => {
          const id = idOf(item);
          if (editingId === id) {
            return (
              <li key={id} className="crud-editing">
                <FieldForm
                  fields={fields}
                  initial={toForm(item)}
                  submitLabel="Save changes"
                  onSubmit={async (body) => { await update(id, body); setEditingId(null); onChange(); }}
                  onCancel={() => setEditingId(null)}
                />
              </li>
            );
          }
          return (
            <li key={id}>
              <span>{summary(item)}</span>
              {canWrite && (
                <span className="row-actions">
                  <button className="small-btn" onClick={() => { setEditingId(id); setAdding(false); }}>Edit</button>
                  <DeleteButton onDelete={async () => { await remove(id); onChange(); }} />
                </span>
              )}
            </li>
          );
        })}
        {items.length === 0 && <li className="muted">Nothing yet.</li>}
      </ul>
    </section>
  );
}

function FieldForm({
  fields,
  initial,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  fields: CrudField[];
  initial: Record<string, string>;
  submitLabel: string;
  onSubmit: (body: Record<string, string>) => Promise<void>;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<Record<string, string>>(initial);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await onSubmit(form);
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
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
      <div className="inline-form">
        <button className="primary" disabled={busy}>{busy ? "…" : submitLabel}</button>
        <button type="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  );
}

// DeleteButton shows an inline two-step confirm instead of a blocking dialog.
export function DeleteButton({ onDelete, label = "Delete" }: { onDelete: () => Promise<void>; label?: string }) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function run() {
    setBusy(true);
    setErr("");
    try {
      await onDelete();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Failed");
      setConfirming(false);
    } finally {
      setBusy(false);
    }
  }

  if (!confirming) {
    return (
      <span className="inline-form">
        {err && <span className="error small">{err}</span>}
        <button className="small-btn danger" onClick={() => setConfirming(true)}>{label}</button>
      </span>
    );
  }
  return (
    <span className="inline-form">
      <span className="muted small">Sure?</span>
      <button className="small-btn danger" disabled={busy} onClick={run}>{busy ? "…" : "Confirm"}</button>
      <button className="small-btn" onClick={() => setConfirming(false)}>Cancel</button>
    </span>
  );
}
