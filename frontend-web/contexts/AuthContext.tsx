"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { authApi } from "@/lib/api-client";
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

  // Restore session from localStorage on mount
  useEffect(() => {
    try {
      const raw = localStorage.getItem(USER_KEY);
      const accessToken = localStorage.getItem(TOKEN_KEY);
      const refreshToken = localStorage.getItem(REFRESH_KEY);
      const expiresAt = Number(localStorage.getItem(EXPIRES_KEY) ?? "0");

      if (raw && accessToken) {
        const user: User = JSON.parse(raw);
        setState({
          user,
          tokens: { accessToken, refreshToken: refreshToken ?? "", expiresAt },
          loading: false,
        });
        return;
      }
    } catch {
      // ignore parse errors
    }
    setState((s) => ({ ...s, loading: false }));
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
