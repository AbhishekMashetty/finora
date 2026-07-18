"use client";

// Client-side auth context for Phase 0.
//
// Known simplification: this only checks for the *presence* of a token in
// localStorage on mount — it does not validate the token's signature or
// expiry client-side (that's enforced server-side by the gateway on every
// real request; a stale token just means the first API call 401s and gets
// silently refreshed/redirected, see lib/api.ts). Route protection for
// /dashboard/* is a client-side effect, not Next.js middleware — see
// frontend/README.md "Known simplifications" for the plan to harden this
// with httpOnly cookies + middleware.ts in a later phase.

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  apiFetch,
  clearTokens,
  getAccessToken,
  getRefreshToken,
  setTokens,
} from "./api";

interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (accessToken: string, refreshToken: string) => void;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Reading localStorage (a browser-only external system) must happen
    // post-mount, not during render, to avoid an SSR/client mismatch — this
    // is exactly the "synchronize with an external system" case effects are
    // for, even though it looks like the generic setState-in-effect smell.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsAuthenticated(!!getAccessToken());
    setIsLoading(false);
  }, []);

  function login(accessToken: string, refreshToken: string) {
    setTokens(accessToken, refreshToken);
    setIsAuthenticated(true);
  }

  async function logout() {
    const refreshToken = getRefreshToken();
    try {
      if (refreshToken) {
        await apiFetch("/api/v1/auth/logout", {
          method: "POST",
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
      }
    } catch {
      // Best-effort: even if the server call fails (network error, token
      // already expired, etc.) we still clear local state and send the
      // user to /login below.
    } finally {
      clearTokens();
      setIsAuthenticated(false);
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
    }
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
