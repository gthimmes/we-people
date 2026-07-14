import { Fragment, FormEvent, useEffect, useState } from "react";
import {
  api,
  ApiError,
  Approval,
  LeaveBalance,
  LeaveType,
  ListResponse,
  TimeOffRequest,
  Worker,
} from "../api";

// An approval enriched with the underlying time-off details for display.
interface InboxItem {
  approval: Approval;
  requester: string;
  detail?: TimeOffRequest;
}

export default function TimeOff() {
  const [types, setTypes] = useState<LeaveType[]>([]);
  const [balances, setBalances] = useState<LeaveBalance[]>([]);
  const [requests, setRequests] = useState<TimeOffRequest[]>([]);
  const [inbox, setInbox] = useState<InboxItem[]>([]);
  const [noWorker, setNoWorker] = useState(false);
  const [loading, setLoading] = useState(true);

  async function loadSelf() {
    try {
      const [b, r] = await Promise.all([
        api.get<{ data: LeaveBalance[] }>("/time-off/balances"),
        api.get<{ data: TimeOffRequest[] }>("/time-off/requests"),
      ]);
      setBalances(b.data);
      setRequests(r.data);
      setNoWorker(false);
    } catch (e) {
      // Users not linked to a worker (e.g. a pure admin) have no self-service.
      if (e instanceof ApiError && e.code === "no_worker") setNoWorker(true);
    }
  }

  async function loadInbox() {
    const approvals = (await api.get<{ data: Approval[] }>("/approvals")).data;
    // Map worker ids -> names, and pull the time-off detail for each approval.
    let workerName: Record<string, string> = {};
    try {
      const ws = (await api.get<ListResponse<Worker>>("/workers?limit=200")).data;
      workerName = Object.fromEntries(ws.map((w) => [w.id, `${w.first_name} ${w.last_name}`]));
    } catch {
      /* viewer may lack worker:read; fall back to ids */
    }
    const items = await Promise.all(
      approvals.map(async (a): Promise<InboxItem> => {
        let detail: TimeOffRequest | undefined;
        if (a.requester_worker_id) {
          try {
            const rs = (await api.get<{ data: TimeOffRequest[] }>(`/time-off/requests?worker_id=${a.requester_worker_id}`)).data;
            detail = rs.find((r) => r.approval_request_id === a.id);
          } catch {
            /* ignore */
          }
        }
        return {
          approval: a,
          requester: (a.requester_worker_id && workerName[a.requester_worker_id]) || "Someone",
          detail,
        };
      })
    );
    setInbox(items);
  }

  async function reloadAll() {
    setLoading(true);
    setTypes((await api.get<{ data: LeaveType[] }>("/time-off/leave-types")).data);
    await Promise.all([loadSelf(), loadInbox()]);
    setLoading(false);
  }

  useEffect(() => {
    reloadAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function decide(id: string, approve: boolean) {
    await api.post(`/approvals/${id}/decide`, { approve });
    await reloadAll();
  }

  if (loading) return <div className="muted">Loading…</div>;

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Time off</h1>
          <p className="muted">Request leave and approve your team's requests</p>
        </div>
      </header>

      {inbox.length > 0 && (
        <section className="card">
          <h3>Awaiting your approval</h3>
          <ul className="list">
            {inbox.map(({ approval, requester, detail }) => (
              <li key={approval.id}>
                <span>
                  <strong>{requester}</strong>
                  {detail ? (
                    <span className="muted">
                      {" "}· {detail.leave_type_name} · {detail.hours}h · {detail.start_date.slice(0, 10)}
                      {detail.end_date !== detail.start_date ? `–${detail.end_date.slice(0, 10)}` : ""}
                    </span>
                  ) : (
                    <span className="muted"> · {approval.request_type}</span>
                  )}
                  {detail?.reason && <div className="muted small">{detail.reason}</div>}
                </span>
                <span className="inline-form">
                  <button className="primary small-btn" onClick={() => decide(approval.id, true)}>Approve</button>
                  <button className="small-btn danger" onClick={() => decide(approval.id, false)}>Reject</button>
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {noWorker ? (
        <div className="card muted">
          Your login isn't linked to an employee record, so you don't have personal balances.
          You can still approve your team's requests above.
        </div>
      ) : (
        <div className="detail-grid">
          <section className="card">
            <h3>My balances</h3>
            <dl className="fields">
              {balances.map((b) => (
                <Fragment key={b.leave_type_id}>
                  <dt>{b.leave_type_name}</dt>
                  <dd>{b.balance_hours} h</dd>
                </Fragment>
              ))}
            </dl>
          </section>

          <RequestForm types={types} onSubmitted={reloadAll} />
        </div>
      )}

      {!noWorker && (
        <section className="card">
          <h3>My requests</h3>
          {requests.length === 0 ? (
            <p className="muted">No requests yet.</p>
          ) : (
            <div className="card no-pad">
              <table>
                <thead>
                  <tr><th>Type</th><th>Dates</th><th>Hours</th><th>Reason</th><th>Status</th></tr>
                </thead>
                <tbody>
                  {requests.map((r) => (
                    <tr key={r.id}>
                      <td>{r.leave_type_name}</td>
                      <td className="muted">{r.start_date.slice(0, 10)}{r.end_date !== r.start_date ? ` – ${r.end_date.slice(0, 10)}` : ""}</td>
                      <td>{r.hours}</td>
                      <td className="muted">{r.reason || "—"}</td>
                      <td><span className={`badge badge-${statusClass(r.status)}`}>{r.status}</span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function statusClass(s: string) {
  return s === "approved" ? "active" : s === "rejected" ? "terminated" : "pending";
}

function RequestForm({ types, onSubmitted }: { types: LeaveType[]; onSubmitted: () => void }) {
  const [leaveTypeId, setLeaveTypeId] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [hours, setHours] = useState("8");
  const [reason, setReason] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await api.post("/time-off/requests", {
        leave_type_id: leaveTypeId || types[0]?.id,
        start_date: startDate,
        end_date: endDate || startDate,
        hours: Number(hours),
        reason,
      });
      setStartDate("");
      setEndDate("");
      setReason("");
      onSubmitted();
    } catch (e2) {
      setErr(e2 instanceof ApiError ? e2.message : "Could not submit request");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h3>Request time off</h3>
      <form className="stack" onSubmit={submit}>
        <label>
          Leave type
          <select value={leaveTypeId} onChange={(e) => setLeaveTypeId(e.target.value)}>
            {types.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
          </select>
        </label>
        <div className="two-col">
          <label>Start date<input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required /></label>
          <label>End date<input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} /></label>
        </div>
        <label>Hours<input type="number" min="1" step="1" value={hours} onChange={(e) => setHours(e.target.value)} required /></label>
        <label>Reason<input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Optional" /></label>
        {err && <div className="error">{err}</div>}
        <button className="primary" disabled={busy}>{busy ? "Submitting…" : "Submit request"}</button>
      </form>
    </section>
  );
}
