"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { api, ApiError, setUnauthorizedHandler } from "@/lib/api";
import type { Staff } from "@/lib/types";

const STORAGE_KEY = "triagely.token";

type AuthState =
  | { status: "loading" }
  | { status: "anon" }
  | { status: "authed"; token: string; staff: Staff };

interface AuthContextValue {
  state: AuthState;
  login: (email: string, password: string) => Promise<void>;
  register: (input: { email: string; password: string; full_name: string }) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// localStorage can throw (private mode, blocked storage), so never trust it.
function readToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

function writeToken(token: string | null) {
  try {
    if (token) localStorage.setItem(STORAGE_KEY, token);
    else localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Without storage, the login only lasts until the page is closed.
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "loading" });

  const logout = useCallback(() => {
    writeToken(null);
    setState({ status: "anon" });
  }, []);

  // Any 401 on a logged-in request means the token is dead, so log out.
  useEffect(() => {
    setUnauthorizedHandler(logout);
    return () => setUnauthorizedHandler(null);
  }, [logout]);

  // On page load: if a saved token is still good, we are logged in.
  useEffect(() => {
    const controller = new AbortController();

    async function restore() {
      const token = readToken();
      if (!token) {
        setState({ status: "anon" });
        return;
      }
      try {
        const { staff } = await api.me(token, controller.signal);
        setState({ status: "authed", token, staff });
      } catch (err) {
        if (controller.signal.aborted) return;
        if (err instanceof ApiError && err.status === 401) writeToken(null);
        // If the server was just unreachable, keep the token and show the login page.
        setState({ status: "anon" });
      }
    }

    void restore();
    return () => controller.abort();
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const res = await api.login(email, password);
    writeToken(res.token);
    setState({ status: "authed", token: res.token, staff: res.staff });
  }, []);

  const register = useCallback(
    async (input: { email: string; password: string; full_name: string }) => {
      const res = await api.register(input);
      writeToken(res.token);
      setState({ status: "authed", token: res.token, staff: res.staff });
    },
    [],
  );

  const value = useMemo(
    () => ({ state, login, register, logout }),
    [state, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}

/** For pages behind the login gate. The token and staff are always there. */
export function useAuthed(): { token: string; staff: Staff; logout: () => void } {
  const { state, logout } = useAuth();
  if (state.status !== "authed") {
    throw new Error("useAuthed must be used on a page behind <AuthGate>");
  }
  return { token: state.token, staff: state.staff, logout };
}
