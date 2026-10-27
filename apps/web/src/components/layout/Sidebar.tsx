"use client";

import React, { useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import {
  Shield,
  Home,
  Info,
  Calendar,
  FolderKanban,
  Heart,
  Store,
  Briefcase,
  Users,
  BarChart3,
  Camera,
  ShieldAlert,
  LayoutDashboard,
  UserCheck,
  Award,
  Bell,
  ChevronLeft,
  ChevronRight,
  X,
  FileText,
  Boxes,
  Lock,
  Settings,
} from "lucide-react";

interface SidebarProps {
  collapsed: boolean;
  setCollapsed: (collapsed: boolean) => void;
  mobileOpen: boolean;
  setMobileOpen: (open: boolean) => void;
}

interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: string;
  variant?: "default" | "alert";
  exact?: boolean;
}

interface NavSection {
  title: string;
  items: NavItem[];
  requiresAuth?: boolean;
  requiresCoordinator?: boolean;
}

export function Sidebar({
  collapsed,
  setCollapsed,
  mobileOpen,
  setMobileOpen,
}: SidebarProps) {
  const pathname = usePathname();
  const { user, isCoordinator } = useAuth();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const mobileDrawerRef = useRef<HTMLDivElement>(null);

  // Close mobile drawer on route change
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname, setMobileOpen]);

  // Handle ESC key to close mobile drawer
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && mobileOpen) {
        setMobileOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [mobileOpen, setMobileOpen]);

  // Focus close button when mobile drawer opens
  useEffect(() => {
    if (mobileOpen && closeButtonRef.current) {
      closeButtonRef.current.focus();
    }
  }, [mobileOpen]);

  const isActive = (item: NavItem) => {
    if (item.exact || item.href === "/") {
      return pathname === item.href;
    }
    return pathname.startsWith(item.href);
  };

  const navSections: NavSection[] = [
    {
      title: "Platform",
      items: [
        { href: "/", label: "Home", icon: Home, exact: true },
        { href: "/about", label: "About Movement", icon: Info },
      ],
    },
    {
      title: "Civic Operations",
      items: [
        { href: "/activities", label: "Cleanups & Events", icon: Calendar },
        { href: "/projects", label: "Community Projects", icon: FolderKanban },
        { href: "/foundation", label: "Ortse Foundation", icon: Heart },
      ],
    },
    {
      title: "Economy & Careers",
      items: [
        { href: "/marketplace", label: "Marketplace", icon: Store },
        { href: "/careers", label: "Careers & Jobs", icon: Briefcase },
      ],
    },
    {
      title: "Governance & Transparency",
      items: [
        { href: "/leadership", label: "Leadership Directory", icon: Users },
        { href: "/reports", label: "Public Reports", icon: BarChart3 },
        { href: "/gallery", label: "Media Gallery", icon: Camera },
        {
          href: "/report-incident",
          label: "Report Incident",
          icon: ShieldAlert,
          badge: "Confidential",
          variant: "alert",
        },
      ],
    },
    {
      title: "Member Workspace",
      requiresAuth: true,
      items: [
        { href: "/member", label: "Member Dashboard", icon: UserCheck, exact: true },
        { href: "/member/participations", label: "My Participations", icon: Award },
        { href: "/notifications", label: "Notifications", icon: Bell },
      ],
    },
    {
      title: "Administration",
      requiresCoordinator: true,
      items: [
        { href: "/admin", label: "Overview", icon: LayoutDashboard, exact: true },
        { href: "/admin/reviews", label: "Members", icon: Users },
        { href: "/admin/leadership", label: "Leadership", icon: Shield },
        { href: "/admin/activities", label: "Activities", icon: Calendar },
        { href: "/admin/finance", label: "Finance", icon: FileText },
        { href: "/admin/marketplace", label: "Marketplace & Vendors", icon: Store },
        { href: "/admin/reports", label: "Reports & Transparency", icon: BarChart3 },
        { href: "/admin/inventory", label: "Supply Chain & Warehouses", icon: Boxes },
        { href: "/admin/safeguarding", label: "Safeguarding Cases", icon: ShieldAlert, badge: "Restricted" },
        { href: "/admin/audit", label: "System Settings", icon: Settings },
      ],
    },
  ];

  const visibleSections = navSections.filter((section) => {
    if (section.requiresAuth && !user) return false;
    if (section.requiresCoordinator && !isCoordinator) return false;
    return true;
  });

  const renderNavContent = (isMobile = false) => (
    <div className="flex flex-col h-full">
      {/* Brand Header */}
      <div className="flex items-center justify-between h-16 px-4 border-b border-gray-800 bg-gray-950/40">
        <Link
          href="/"
          className="flex items-center gap-3 focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none rounded-lg p-1"
          aria-label="Ratels Worldwide Home"
          onClick={() => isMobile && setMobileOpen(false)}
        >
          <div className="w-9 h-9 rounded-lg bg-emerald-800 text-emerald-100 flex items-center justify-center shadow-sm shrink-0">
            <Shield className="w-5 h-5 text-emerald-200" aria-hidden="true" />
          </div>
          {(!collapsed || isMobile) && (
            <div className="overflow-hidden">
              <span className="font-extrabold text-white text-base tracking-tight block leading-tight truncate">
                Ratels Worldwide
              </span>
              <span className="text-[10px] text-emerald-400 font-semibold tracking-wider uppercase block truncate">
                Operations & Governance
              </span>
            </div>
          )}
        </Link>

        {isMobile ? (
          <button
            ref={closeButtonRef}
            type="button"
            onClick={() => setMobileOpen(false)}
            className="p-2 text-gray-400 hover:text-white rounded-lg hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none transition"
            aria-label="Close navigation menu"
          >
            <X className="w-5 h-5" aria-hidden="true" />
          </button>
        ) : (
          <button
            type="button"
            onClick={() => setCollapsed(!collapsed)}
            className="p-1.5 text-gray-400 hover:text-white rounded-lg hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none transition"
            aria-controls="desktop-primary-sidebar"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {collapsed ? (
              <ChevronRight className="w-4 h-4" aria-hidden="true" />
            ) : (
              <ChevronLeft className="w-4 h-4" aria-hidden="true" />
            )}
          </button>
        )}
      </div>

      {/* Navigation Sections */}
      <nav
        className="flex-1 overflow-y-auto px-3 py-4 space-y-6 scrollbar-thin"
        aria-label="Sidebar navigation"
      >
        {visibleSections.map((section, idx) => (
          <div key={section.title || idx} className="space-y-1">
            {(!collapsed || isMobile) && (
              <h2 className="px-3 text-[11px] font-bold text-gray-400 uppercase tracking-wider mb-2 select-none">
                {section.title}
              </h2>
            )}
            <ul className="space-y-1" role="list">
              {section.items.map((item) => {
                const active = isActive(item);
                const Icon = item.icon;
                const isAlert = item.variant === "alert";

                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      onClick={() => isMobile && setMobileOpen(false)}
                      className={`group flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none ${
                        active
                          ? "bg-emerald-900/60 text-white font-semibold border-l-4 border-emerald-400 shadow-sm"
                          : isAlert
                          ? "text-amber-300 hover:bg-amber-950/40 hover:text-amber-200 border border-amber-900/30"
                          : "text-gray-300 hover:bg-gray-800 hover:text-white"
                      }`}
                      aria-current={active ? "page" : undefined}
                      title={collapsed && !isMobile ? item.label : undefined}
                    >
                      <Icon
                        className={`w-5 h-5 shrink-0 ${
                          active
                            ? "text-emerald-300"
                            : isAlert
                            ? "text-amber-400"
                            : "text-gray-400 group-hover:text-gray-200"
                        }`}
                        aria-hidden="true"
                      />
                      {(!collapsed || isMobile) && (
                        <span className="flex-1 truncate">{item.label}</span>
                      )}
                      {(!collapsed || isMobile) && item.badge && (
                        <span
                          className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${
                            isAlert
                              ? "bg-amber-950 text-amber-300 border border-amber-800"
                              : "bg-emerald-950 text-emerald-300 border border-emerald-800"
                          }`}
                        >
                          {item.badge}
                        </span>
                      )}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      {/* Footer Branding & Disclaimer Note */}
      {(!collapsed || isMobile) && (
        <div className="p-3 border-t border-gray-800 text-[11px] text-gray-500 bg-gray-950/30 leading-snug">
          <div className="flex items-center gap-1.5 text-gray-400 font-semibold mb-1">
            <Lock className="w-3.5 h-3.5 text-emerald-500" aria-hidden="true" />
            <span>Ratels Worldwide Platform</span>
          </div>
          <p className="text-[10px] text-gray-500">
            Certified Civic & Public Transparency Framework.
          </p>
        </div>
      )}
    </div>
  );

  return (
    <>
      {/* Desktop Sticky Sidebar */}
      <aside
        id="desktop-primary-sidebar"
        className={`hidden lg:flex flex-col fixed inset-y-0 left-0 z-40 bg-gray-900 text-gray-200 border-r border-gray-800 transition-all duration-200 ease-in-out ${
          collapsed ? "w-20" : "w-64 xl:w-72"
        }`}
        aria-label="Primary Platform Navigation"
      >
        {renderNavContent(false)}
      </aside>

      {/* Mobile Drawer (with Backdrop & Accessibility Trap) */}
      {mobileOpen && (
        <div
          id="mobile-navigation-drawer"
          className="lg:hidden fixed inset-0 z-50 flex"
          role="dialog"
          aria-modal="true"
          aria-label="Navigation drawer"
        >
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
            onClick={() => setMobileOpen(false)}
            aria-hidden="true"
          />

          {/* Drawer content */}
          <div
            ref={mobileDrawerRef}
            className="relative flex flex-col w-full max-w-xs bg-gray-900 text-gray-200 shadow-2xl z-10 animate-in slide-in-from-left duration-200"
          >
            {renderNavContent(true)}
          </div>
        </div>
      )}
    </>
  );
}
