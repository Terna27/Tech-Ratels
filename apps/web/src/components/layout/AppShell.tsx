"use client";

import React, { useState, useEffect } from "react";
import { Sidebar } from "./Sidebar";
import { Header } from "./Header";
import { Footer } from "./Footer";

interface AppShellProps {
  children: React.ReactNode;
}

const SIDEBAR_PREF_KEY = "ratels_sidebar_collapsed_v1";

export function AppShell({ children }: AppShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isClient, setIsClient] = useState(false);

  // Restore sidebar state from localStorage on client mount
  useEffect(() => {
    setIsClient(true);
    try {
      const saved = localStorage.getItem(SIDEBAR_PREF_KEY);
      if (saved !== null) {
        setCollapsed(saved === "true");
      }
    } catch {
      // Ignore localStorage access errors
    }
  }, []);

  // Persist sidebar collapsed state
  const handleSetCollapsed = (val: boolean) => {
    setCollapsed(val);
    try {
      localStorage.setItem(SIDEBAR_PREF_KEY, String(val));
    } catch {
      // Ignore quota errors
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col text-gray-900 antialiased">
      {/* WCAG 2.2 AA Skip Link */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-emerald-800 focus:text-white focus:rounded-lg focus:shadow-xl focus:outline-none focus:ring-2 focus:ring-emerald-400 focus:ring-offset-2 font-bold text-sm"
      >
        Skip to main content
      </a>

      {/* Primary Left Sidebar */}
      <Sidebar
        collapsed={collapsed}
        setCollapsed={handleSetCollapsed}
        mobileOpen={mobileOpen}
        setMobileOpen={setMobileOpen}
      />

      {/* Main Content Column (dynamically padded by sidebar width on desktop) */}
      <div
        className={`flex-1 flex flex-col min-w-0 transition-all duration-200 ease-in-out ${
          collapsed ? "lg:pl-20" : "lg:pl-64 xl:pl-72"
        }`}
      >
        {/* Top Utility Header */}
        <Header
          collapsed={collapsed}
          setCollapsed={handleSetCollapsed}
          mobileOpen={mobileOpen}
          setMobileOpen={setMobileOpen}
        />

        {/* Primary Page Content */}
        <main
          id="main-content"
          tabIndex={-1}
          className="flex-1 w-full max-w-full outline-none focus:outline-none"
        >
          {children}
        </main>

        {/* Rebranded Ratels Worldwide Footer */}
        <Footer />
      </div>
    </div>
  );
}
