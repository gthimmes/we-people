import { useEffect, useState } from "react";
import { api, OrgNode } from "../api";

function initials(n: OrgNode) {
  return `${n.first_name[0] ?? ""}${n.last_name[0] ?? ""}`.toUpperCase();
}

function Node({ node }: { node: OrgNode }) {
  return (
    <li>
      <div className="node">
        <div className="avatar">{initials(node)}</div>
        <div>
          <div className="node-name">
            {node.first_name} {node.last_name}
          </div>
          <div className="muted small">{node.title ?? "—"}</div>
        </div>
      </div>
      {node.reports.length > 0 && (
        <ul>
          {node.reports.map((r) => (
            <Node key={r.worker_id} node={r} />
          ))}
        </ul>
      )}
    </li>
  );
}

export default function OrgChart() {
  const [roots, setRoots] = useState<OrgNode[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .get<{ data: OrgNode[] }>("/org-chart")
      .then((r) => setRoots(r.data))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <header className="page-header">
        <div>
          <h1>Org chart</h1>
          <p className="muted">Reporting hierarchy from current assignments</p>
        </div>
      </header>

      {loading ? (
        <div className="muted">Loading…</div>
      ) : roots.length === 0 ? (
        <div className="card muted">No assignments yet.</div>
      ) : (
        <div className="card org-tree">
          <ul className="tree-root">
            {roots.map((r) => (
              <Node key={r.worker_id} node={r} />
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
