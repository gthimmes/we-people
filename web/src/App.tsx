import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./auth";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import Directory from "./pages/Directory";
import WorkerDetail from "./pages/WorkerDetail";
import OrgManage from "./pages/OrgManage";
import OrgChart from "./pages/OrgChart";
import TimeOff from "./pages/TimeOff";
import Onboarding from "./pages/Onboarding";
import Analytics from "./pages/Analytics";
import Team from "./pages/Team";
import NotificationBell from "./components/NotificationBell";
import HelpWidget from "./components/HelpWidget";

export default function App() {
  const { me, loading, logout } = useAuth();

  if (loading) return <div className="center muted">Loading…</div>;
  if (!me) return <Login />;

  const canManageUsers = me.permissions.includes("user:read");

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          We<span>People</span>
        </div>
        <nav>
          <NavLink to="/dashboard">Home</NavLink>
          <NavLink to="/directory">People</NavLink>
          <NavLink to="/org">Organization</NavLink>
          <NavLink to="/org-chart">Org chart</NavLink>
          <NavLink to="/time-off">Time off</NavLink>
          <NavLink to="/onboarding">Onboarding</NavLink>
          <NavLink to="/analytics">Analytics</NavLink>
          {canManageUsers && <NavLink to="/team">Team &amp; access</NavLink>}
        </nav>
        <div className="sidebar-footer">
          <div className="muted small">{me.email}</div>
          <button className="link" onClick={logout}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <NotificationBell />
        </header>
        <div className="content">
          <Routes>
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/directory" element={<Directory />} />
            <Route path="/people/:id" element={<WorkerDetail />} />
            <Route path="/org" element={<OrgManage />} />
            <Route path="/org-chart" element={<OrgChart />} />
            <Route path="/time-off" element={<TimeOff />} />
            <Route path="/onboarding" element={<Onboarding />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/team" element={<Team />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </div>
      </main>
      <HelpWidget />
    </div>
  );
}
