"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { useCart } from "@/lib/cart-context";
import { NotificationBell } from "@/components/notifications/NotificationBell";
import {
  Menu,
  ShoppingCart,
  ShieldAlert,
  User as UserIcon,
  LogOut,
  LayoutDashboard,
  Settings,
  ChevronDown,
  Shield,
  PanelLeftClose,
  PanelLeft,
  ShoppingBag,
  Store,
} from "lucide-react";
import { Badge } from "@/components/ui/Badge";

interface HeaderProps {
  collapsed: boolean;
  setCollapsed: (collapsed: boolean) => void;
  mobileOpen: boolean;
  setMobileOpen: (open: boolean) => void;
}

export function Header({
  collapsed,
  setCollapsed,
  mobileOpen,
  setMobileOpen,
}: HeaderProps) {
  const pathname = usePathname();
  const { user, isCoordinator, logout } = useAuth();
  const { totalItems } = useCart();
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    const handleOutsideClick = (event: MouseEvent) => {
      if (
        userMenuRef.current &&
        !userMenuRef.current.contains(event.target as Node)
      ) {
        setUserMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleOutsideClick);
    return () => document.removeEventListener("mousedown", handleOutsideClick);
  }, []);

  // Close user dropdown on route change
  useEffect(() => {
    setUserMenuOpen(false);
  }, [pathname]);

  // Determine page title / breadcrumb from pathname
  const getSectionTitle = () => {
    if (pathname === "/") return "Overview";
    if (pathname.startsWith("/about")) return "About Movement";
    if (pathname.startsWith("/activities")) return "Cleanups & Civic Events";
    if (pathname.startsWith("/projects")) return "Community Projects";
    if (pathname.startsWith("/foundation")) return "Ortse Foundation";
    if (pathname.startsWith("/marketplace")) return "Marketplace";
    if (pathname.startsWith("/careers")) return "Careers & Employment";
    if (pathname.startsWith("/leadership")) return "Leadership Directory";
    if (pathname.startsWith("/reports")) return "Transparency & Public Reports";
    if (pathname.startsWith("/gallery")) return "Media Gallery";
    if (pathname.startsWith("/report-incident")) return "Confidential Incident Reporting";
    if (pathname.startsWith("/admin")) return "Administration Workspace";
    if (pathname.startsWith("/member")) return "Member Workspace";
    if (pathname.startsWith("/notifications")) return "Notifications";
    if (pathname.startsWith("/settings")) return "Settings";
    if (pathname.startsWith("/login")) return "Sign In";
    if (pathname.startsWith("/register")) return "Join Movement";
    return "Ratels Worldwide";
  };

  return (
    <header className="sticky top-0 z-30 bg-white/95 backdrop-blur-md border-b border-gray-200">
      {/* Top Banner: Rebranded Ratels Worldwide */}
      <div className="bg-emerald-950 text-emerald-100 text-xs px-4 py-1 text-center font-medium">
        <span>Ratels Worldwide Platform • Civic Action, Transparency & Humanitarian Service</span>
      </div>

      <div className="px-3 sm:px-4 lg:px-6">
        <div className="flex items-center justify-between h-14 gap-2">
          {/* Left: Mobile Toggle & Breadcrumb Title */}
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            {/* Mobile Hamburger Toggle */}
            <button
              type="button"
              onClick={() => setMobileOpen(!mobileOpen)}
              className="lg:hidden p-2 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none transition shrink-0"
              aria-label="Open navigation menu"
              aria-expanded={mobileOpen}
              aria-controls="mobile-navigation-drawer"
            >
              <Menu className="w-5 h-5" aria-hidden="true" />
            </button>

            {/* Desktop Sidebar Toggle */}
            <button
              type="button"
              onClick={() => setCollapsed(!collapsed)}
              className="hidden lg:flex p-1.5 text-gray-500 hover:text-gray-800 hover:bg-gray-100 rounded-lg focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none transition shrink-0"
              aria-controls="desktop-primary-sidebar"
              aria-expanded={!collapsed}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
              title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? (
                <PanelLeft className="w-5 h-5" aria-hidden="true" />
              ) : (
                <PanelLeftClose className="w-5 h-5" aria-hidden="true" />
              )}
            </button>

            {/* Section Breadcrumb */}
            <div className="flex items-center gap-1.5 text-sm min-w-0">
              <span className="hidden md:inline font-semibold text-gray-500 shrink-0">
                Ratels Worldwide
              </span>
              <span className="hidden md:inline text-gray-300 shrink-0">/</span>
              <h1 className="font-bold text-gray-900 text-xs sm:text-sm tracking-tight truncate">
                {getSectionTitle()}
              </h1>
            </div>
          </div>

          {/* Right Utilities: Incident Report, Cart, Notifications, User Menu */}
          <div className="flex items-center gap-1.5 sm:gap-2.5 shrink-0">
            {/* Confidential Incident Report Link */}
            <Link
              href="/report-incident"
              className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-bold text-amber-900 bg-amber-50 hover:bg-amber-100 border border-amber-200 transition focus-visible:ring-2 focus-visible:ring-amber-500 focus-visible:outline-none"
              title="Report Incident (Confidential)"
            >
              <ShieldAlert className="w-4 h-4 text-amber-700 shrink-0" aria-hidden="true" />
              <span className="hidden md:inline">Report Incident</span>
            </Link>

            {/* Shopping Cart Button */}
            <Link
              href="/marketplace/cart"
              className="relative p-2 text-gray-700 hover:text-emerald-900 hover:bg-gray-100 rounded-lg transition focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none"
              title="Shopping Cart"
              aria-label={`Shopping cart with ${totalItems} items`}
            >
              <ShoppingCart className="w-5 h-5" aria-hidden="true" />
              {totalItems > 0 && (
                <span className="absolute -top-1 -right-1 bg-emerald-700 text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center">
                  {totalItems > 99 ? "99+" : totalItems}
                </span>
              )}
            </Link>

            {/* Notifications (when authenticated) */}
            {user && <NotificationBell />}

            {/* User Profile / Auth Area */}
            {user ? (
              <div className="relative" ref={userMenuRef}>
                <button
                  type="button"
                  onClick={() => setUserMenuOpen(!userMenuOpen)}
                  className="flex items-center gap-2 p-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none transition"
                  aria-expanded={userMenuOpen}
                  aria-haspopup="menu"
                  aria-label="User account menu"
                >
                  <div className="w-7 h-7 rounded-full bg-emerald-800 text-white text-xs font-bold flex items-center justify-center">
                    {user.display_name.charAt(0).toUpperCase()}
                  </div>
                  <span className="hidden md:inline text-xs font-semibold text-gray-800 max-w-[120px] truncate">
                    {user.display_name.split(" ")[0]}
                  </span>
                  <ChevronDown className="w-3.5 h-3.5 text-gray-500" aria-hidden="true" />
                </button>

                {/* Dropdown Menu */}
                {userMenuOpen && (
                  <div
                    className="absolute right-0 mt-2 w-64 bg-white rounded-xl shadow-lg border border-gray-200 py-1.5 z-50 animate-in fade-in-50 duration-150"
                    role="menu"
                    aria-orientation="vertical"
                  >
                    {/* User Info Header */}
                    <div className="px-4 py-2.5 border-b border-gray-100 bg-gray-50/60">
                      <p className="text-sm font-bold text-gray-900 truncate">
                        {user.display_name}
                      </p>
                      <p className="text-xs text-gray-500 truncate">
                        {user.email || user.phone}
                      </p>
                      <div className="mt-1.5 flex items-center gap-1.5">
                        <Badge variant="success" size="sm">
                          {user.primary_role.replace("_", " ")}
                        </Badge>
                        <span className="text-[10px] text-gray-500 font-mono">
                          {user.membership_reference}
                        </span>
                      </div>
                    </div>

                    {/* Navigation Items */}
                    <div className="py-1">
                      <Link
                        href="/member"
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900"
                        role="menuitem"
                      >
                        <UserIcon className="w-4 h-4 text-emerald-700" aria-hidden="true" />
                        <span>Member Dashboard</span>
                      </Link>

                      <Link
                        href="/member/profile"
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900"
                        role="menuitem"
                      >
                        <Settings className="w-4 h-4 text-gray-500" aria-hidden="true" />
                        <span>Profile & Contact</span>
                      </Link>

                      <Link
                        href="/marketplace/orders"
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900"
                        role="menuitem"
                      >
                        <ShoppingBag className="w-4 h-4 text-emerald-700" aria-hidden="true" />
                        <span>My Orders</span>
                      </Link>

                      <Link
                        href="/marketplace/seller"
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900"
                        role="menuitem"
                      >
                        <Store className="w-4 h-4 text-emerald-700" aria-hidden="true" />
                        <span>Seller Portal</span>
                      </Link>

                      {isCoordinator && (
                        <Link
                          href="/admin"
                          className="flex items-center gap-2.5 px-4 py-2 text-xs font-bold text-emerald-900 bg-emerald-50/60 hover:bg-emerald-100/70"
                          role="menuitem"
                        >
                          <LayoutDashboard className="w-4 h-4 text-emerald-800" aria-hidden="true" />
                          <span>Admin Workspace</span>
                        </Link>
                      )}
                    </div>

                    {/* Sign Out */}
                    <div className="pt-1 border-t border-gray-100">
                      <button
                        type="button"
                        onClick={async () => {
                          setUserMenuOpen(false);
                          try {
                            await logout();
                          } catch (err: any) {
                            alert("Sign out failed: " + (err?.message || "Please check your network connection."));
                          }
                        }}
                        className="flex items-center gap-2.5 w-full text-left px-4 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-50"
                        role="menuitem"
                      >
                        <LogOut className="w-4 h-4 text-rose-600" aria-hidden="true" />
                        <span>Sign Out</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Link
                  href="/login"
                  className="px-3 py-1.5 text-xs font-semibold text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none"
                >
                  Sign In
                </Link>
                <Link
                  href="/register"
                  className="px-3.5 py-1.5 text-xs font-semibold text-white bg-emerald-800 hover:bg-emerald-900 rounded-lg shadow-sm transition focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:outline-none"
                >
                  Join Movement
                </Link>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
