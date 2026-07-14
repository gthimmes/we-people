// Minimal typed API client for the We People backend.

const ACCESS_KEY = "wp_access";
const REFRESH_KEY = "wp_refresh";

export const tokens = {
  get access() {
    return localStorage.getItem(ACCESS_KEY);
  },
  get refresh() {
    return localStorage.getItem(REFRESH_KEY);
  },
  set(access: string, refresh: string) {
    localStorage.setItem(ACCESS_KEY, access);
    localStorage.setItem(REFRESH_KEY, refresh);
  },
  setAccess(access: string) {
    localStorage.setItem(ACCESS_KEY, access);
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
  },
};

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, string>;
  constructor(status: number, code: string, message: string, details?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

async function raw<T>(method: string, path: string, body?: unknown, auth = true): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (auth && tokens.access) headers["Authorization"] = `Bearer ${tokens.access}`;
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const e = data?.error ?? {};
    throw new ApiError(res.status, e.code ?? "error", e.message ?? res.statusText, e.details);
  }
  return data as T;
}

// request wraps raw with a single transparent refresh-and-retry on 401.
async function request<T>(method: string, path: string, body?: unknown, auth = true): Promise<T> {
  try {
    return await raw<T>(method, path, body, auth);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401 && auth && tokens.refresh) {
      const refreshed = await raw<{ token: Token }>("POST", "/auth/refresh", { refresh_token: tokens.refresh }, false);
      tokens.setAccess(refreshed.token.access_token);
      return await raw<T>(method, path, body, auth);
    }
    throw err;
  }
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown, auth = true) => request<T>("POST", path, body, auth),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
};

// --- Types ---

export interface Token {
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface Worker {
  id: string;
  employee_number: string;
  first_name: string;
  last_name: string;
  preferred_name: string;
  work_email: string;
  phone: string;
  hire_date?: string;
  status: string;
}

export interface Me {
  user_id: string;
  org_id: string;
  email: string;
  permissions: string[];
}

export interface OrgNode {
  worker_id: string;
  first_name: string;
  last_name: string;
  title?: string;
  manager_id?: string;
  reports: OrgNode[];
}

export interface ListResponse<T> {
  data: T[];
  meta: { total: number; limit: number; offset: number };
}

export interface Profile extends Worker {
  personal_email: string;
  date_of_birth?: string;
  position_title?: string;
  department_name?: string;
  location_name?: string;
  manager_id?: string;
  manager_name?: string;
}

export interface LifecycleEvent {
  id: string;
  type: string;
  effective_date: string;
  reason: string;
  created_at: string;
}

export interface Department {
  id: string;
  name: string;
  code: string;
  parent_id?: string;
  cost_center: string;
}

export interface Location {
  id: string;
  name: string;
  city: string;
  region: string;
  country: string;
  timezone: string;
}

export interface Position {
  id: string;
  title: string;
  department_id?: string;
  location_id?: string;
  status: string;
  fte: number;
}

export interface Assignment {
  id: string;
  worker_id: string;
  position_id?: string;
  manager_id?: string;
  effective_date: string;
  is_primary: boolean;
}
