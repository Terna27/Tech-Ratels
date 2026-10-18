"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { User } from "./types";
import { apiRequest, setCsrfToken, clearCsrfToken } from "./api";

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  login: (userData: User, csrfToken?: string) => void;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
  isCoordinator: boolean;
  isAdmin: boolean;
  isSuperAdmin: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Authenticate using HttpOnly cookie credentials
  const refreshUser = async () => {
    try {
      const userData = await apiRequest<User>("/members/me");
      setUser(userData);
    } catch {
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    refreshUser();
  }, []);

  const login = (userData: User, csrfToken?: string) => {
    if (csrfToken) {
      setCsrfToken(csrfToken);
    }
    setUser(userData);
  };

  const logout = async () => {
    try {
      await apiRequest("/auth/logout", { method: "POST" });
    } catch {
      // Ignore network errors on logout
    } finally {
      clearCsrfToken();
      setUser(null);
    }
  };

  const isSuperAdmin = Boolean(
    user && (
      user.primary_role === "super_admin" ||
      user.appointments?.some((a) => a.status === "active" && a.role_type === "super_admin")
    )
  );

  const isAdmin = Boolean(
    user && (
      user.primary_role === "national_admin" ||
      user.primary_role === "super_admin" ||
      user.appointments?.some(
        (a) => a.status === "active" && (a.role_type === "national_admin" || a.role_type === "super_admin")
      )
    )
  );

  const isCoordinator = Boolean(
    user && (
      [
        "super_admin",
        "national_admin",
        "zonal_coordinator",
        "state_coordinator",
        "lga_representative",
      ].includes(user.primary_role) ||
      user.appointments?.some(
        (a) =>
          a.status === "active" &&
          [
            "super_admin",
            "national_admin",
            "zonal_coordinator",
            "state_coordinator",
            "lga_representative",
          ].includes(a.role_type)
      )
    )
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        login,
        logout,
        refreshUser,
        isCoordinator,
        isAdmin,
        isSuperAdmin,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
