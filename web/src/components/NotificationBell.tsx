import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, Notification } from "../api";

// NotificationBell shows an unread badge and a dropdown of recent notifications.
export default function NotificationBell() {
  const navigate = useNavigate();
  const [unread, setUnread] = useState(0);
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Notification[]>([]);
  const ref = useRef<HTMLDivElement>(null);

  async function refreshCount() {
    try {
      const r = await api.get<{ unread: number }>("/notifications/unread-count");
      setUnread(r.unread);
    } catch {
      /* ignore transient errors */
    }
  }

  useEffect(() => {
    refreshCount();
    const t = setInterval(refreshCount, 30000);
    return () => clearInterval(t);
  }, []);

  // Close on outside click.
  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  async function toggle() {
    const next = !open;
    setOpen(next);
    if (next) {
      const r = await api.get<{ data: Notification[] }>("/notifications?limit=15");
      setItems(r.data);
    }
  }

  async function openItem(n: Notification) {
    if (!n.read_at) {
      await api.post(`/notifications/${n.id}/read`);
      refreshCount();
    }
    setOpen(false);
    if (n.link) navigate(n.link);
  }

  async function markAll() {
    await api.post("/notifications/read-all");
    setItems((prev) => prev.map((n) => ({ ...n, read_at: n.read_at ?? new Date().toISOString() })));
    setUnread(0);
  }

  return (
    <div className="bell" ref={ref}>
      <button className="bell-btn" onClick={toggle} aria-label="Notifications">
        <span className="bell-icon">🔔</span>
        {unread > 0 && <span className="bell-badge">{unread > 9 ? "9+" : unread}</span>}
      </button>
      {open && (
        <div className="bell-dropdown">
          <div className="bell-head">
            <strong>Notifications</strong>
            {unread > 0 && <button className="link" onClick={markAll}>Mark all read</button>}
          </div>
          {items.length === 0 ? (
            <div className="muted bell-empty">You're all caught up.</div>
          ) : (
            <ul className="bell-list">
              {items.map((n) => (
                <li key={n.id} className={n.read_at ? "" : "unread"} onClick={() => openItem(n)}>
                  <div className="bell-title">{n.title}</div>
                  {n.body && <div className="muted small">{n.body}</div>}
                  <div className="muted small">{new Date(n.created_at).toLocaleString()}</div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
