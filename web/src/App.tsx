import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./auth";
import Login from "./pages/Login";
import Directory from "./pages/Directory";
import WorkerDetail from "./pages/WorkerDetail";
import OrgManage from "./pages/OrgManage";
import OrgChart from "./pages/OrgChart";
import TimeOff from "./pages/TimeOff";

export default function App() {
  const { me, loading, logout } = useAuth();

  if (loading) return <div className="center muted">Loading…</div>;
  if (!me) return <Login />;

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          We<span>People</span>
        </div>
        <nav>
          <NavLink to="/directory">People</NavLink>
          <NavLink to="/org">Organization</NavLink>
          <NavLink to="/org-chart">Org chart</NavLink>
          <NavLink to="/time-off">Time off</NavLink>
        </nav>
        <div className="sidebar-footer">
          <div className="muted small">{me.email}</div>
          <button className="link" onClick={logout}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="content">
        <Routes>
          <Route path="/directory" element={<Directory />} />
          <Route path="/people/:id" element={<WorkerDetail />} />
          <Route path="/org" element={<OrgManage />} />
          <Route path="/org-chart" element={<OrgChart />} />
          <Route path="/time-off" element={<TimeOff />} />
          <Route path="*" element={<Navigate to="/directory" replace />} />
        </Routes>
      </main>
    </div>
  );
}
