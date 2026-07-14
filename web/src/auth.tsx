import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { api, tokens, Me, Token } from "./api";

interface AuthState {
  me: Me | null;
  loading: boolean;
  login: (slug: string, email: string, password: string) => Promise<void>;
  register: (orgName: string, email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  // On mount, if we have a token, load the current principal.
  useEffect(() => {
    if (!tokens.access) {
      setLoading(false);
      return;
    }
    api
      .get<Me>("/me")
      .then(setMe)
      .catch(() => tokens.clear())
      .finally(() => setLoading(false));
  }, []);

  async function login(slug: string, email: string, password: string) {
    const res = await api.post<{ token: Token }>("/auth/login", { slug, email, password }, false);
    tokens.set(res.token.access_token, res.token.refresh_token);
    setMe(await api.get<Me>("/me"));
  }

  async function register(orgName: string, email: string, password: string) {
    const res = await api.post<{ token: Token }>("/auth/register", { org_name: orgName, email, password }, false);
    tokens.set(res.token.access_token, res.token.refresh_token);
    setMe(await api.get<Me>("/me"));
  }

  function logout() {
    tokens.clear();
    setMe(null);
  }

  return (
    <AuthContext.Provider value={{ me, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
