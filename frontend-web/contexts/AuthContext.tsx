"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { authApi, setOnAuthFailure } from "@/lib/api-client";
// authApi.me() is used on session restore to validate the token against the backend.
import type { AuthTokens, User } from "@/lib/types";

interface AuthState {
  user: User | null;
  tokens: AuthTokens | null;
  loading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  register: (params: {
    email: string;
    password: string;
    fullName: string;
    arena: string;
  }) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const TOKEN_KEY = "gold_access_token";
const REFRESH_KEY = "gold_refresh_token";
const EXPIRES_KEY = "gold_token_expires";
const USER_KEY = "gold_user";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    tokens: null,
    loading: true,
  });

  // Restore session on mount. The cached user is shown optimistically for a fast
  // first paint, but the session is then re-validated against the backend via
  // authApi.me(): the token — not localStorage — is the source of truth. If
  // validation fails the session is cleared. (Authorization itself is always
  // enforced server-side; any client-side role is display-only.)
  useEffect(() => {
    let cancelled = false;
    const accessToken = localStorage.getItem(TOKEN_KEY);
    const refreshToken = localStorage.getItem(REFRESH_KEY);
    const expiresAt = Number(localStorage.getItem(EXPIRES_KEY) ?? "0");

    if (!accessToken) {
      setState((s) => ({ ...s, loading: false }));
      return;
    }

    let cachedUser: User | null = null;
    try {
      const raw = localStorage.getItem(USER_KEY);
      if (raw) cachedUser = JSON.parse(raw) as User;
    } catch {
      cachedUser = null;
    }
    // Optimistic paint with the cached identity while we validate.
    setState({
      user: cachedUser,
      tokens: { accessToken, refreshToken: refreshToken ?? "", expiresAt },
      loading: true,
    });

    (async () => {
      try {
        const res = await authApi.me();
        if (cancelled) return;
        // Merge authoritative fields from the backend over the cached user.
        const authoritative = { ...(cachedUser ?? {}), ...res.data } as User;
        localStorage.setItem(USER_KEY, JSON.stringify(authoritative));
        setState({
          user: authoritative,
          tokens: { accessToken: localStorage.getItem(TOKEN_KEY) ?? accessToken, refreshToken: refreshToken ?? "", expiresAt },
          loading: false,
        });
      } catch {
        if (cancelled) return;
        // Token invalid/expired (after the api-client's refresh-and-retry) — clear.
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(REFRESH_KEY);
        localStorage.removeItem(EXPIRES_KEY);
        localStorage.removeItem(USER_KEY);
        setState({ user: null, tokens: null, loading: false });
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  const persist = useCallback((user: User, tokens: AuthTokens) => {
    localStorage.setItem(TOKEN_KEY, tokens.accessToken);
    localStorage.setItem(REFRESH_KEY, tokens.refreshToken);
    localStorage.setItem(EXPIRES_KEY, String(tokens.expiresAt));
    localStorage.setItem(USER_KEY, JSON.stringify(user));
    setState({ user, tokens, loading: false });
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await authApi.login({ email, password });
      persist(res.data.user, res.data.tokens);
    },
    [persist]
  );

  const register = useCallback(
    async (params: { email: string; password: string; fullName: string; arena: string }) => {
      const res = await authApi.register(params);
      persist(res.data.user, res.data.tokens);
    },
    [persist]
  );

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(REFRESH_KEY);
    localStorage.removeItem(EXPIRES_KEY);
    localStorage.removeItem(USER_KEY);
    setState({ user: null, tokens: null, loading: false });
  }, []);

  // When the api-client's refresh-and-retry ultimately fails (e.g. refresh
  // token expired), clear the user/session here too. The api-client has
  // already removed the tokens from localStorage; this resets in-memory state.
  useEffect(() => {
    setOnAuthFailure(() => {
      localStorage.removeItem(USER_KEY);
      setState({ user: null, tokens: null, loading: false });
    });
    return () => setOnAuthFailure(null);
  }, []);

  return (
    <AuthContext.Provider value={{ ...state, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
