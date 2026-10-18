"use client";

import React, { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import {
  Menu,
  X,
  Shield,
  Calendar,
  Users,
  Heart,
  User as UserIcon,
  LogOut,
  LayoutDashboard,
  CheckCircle2,
  ShoppingCart,
} from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import { useCart } from "@/lib/cart-context";

export function Navbar() {
  const pathname = usePathname();
  const { user, isCoordinator, logout } = useAuth();
  const { totalItems } = useCart();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const navLinks = [
    { href: "/", label: "Home" },
    { href: "/about", label: "About" },
    { href: "/activities", label: "Cleanups & Events" },
    { href: "/marketplace", label: "Ratels Marketplace" },
    { href: "/careers", label: "Ratels Careers" },
    { href: "/leadership", label: "Leadership" },
    { href: "/foundation", label: "Foundation" },
  ];

  const isActive = (path: string) => {
    if (path === "/") return pathname === "/";
    return pathname.startsWith(path);
  };

  return (
    <header className="sticky top-0 z-50 bg-white border-b border-gray-200">
      {/* Disclaimer Top Banner */}
      <div className="bg-emerald-950 text-emerald-100 text-xs px-4 py-1.5 text-center font-medium">
        <span>Provisional styling & configuration • Nigerian Ratels Management System (Phase 1)</span>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16 items-center">
          {/* Logo & Brand */}
          <Link href="/" className="flex items-center gap-2.5">
            <div className="w-10 h-10 rounded-lg bg-emerald-900 flex items-center justify-center text-white font-bold shadow-sm">
              <Shield className="w-5 h-5 text-emerald-300" />
            </div>
            <div>
              <span className="font-bold text-gray-900 text-lg tracking-tight block leading-tight">
                Nigerian Ratels
              </span>
              <span className="text-xs text-emerald-800 font-semibold tracking-wider uppercase block">
                Management System
              </span>
            </div>
          </Link>

          {/* Desktop Navigation Links */}
          <nav className="hidden md:flex items-center gap-1">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={`px-3 py-2 rounded-md text-sm font-medium transition ${
                  isActive(link.href)
                    ? "bg-emerald-50 text-emerald-900 font-semibold"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>

          {/* Right Action Buttons */}
          <div className="hidden md:flex items-center gap-3">
            {/* Shopping Cart Button */}
            <Link
              href="/marketplace/cart"
              className="relative p-2 text-gray-700 hover:text-emerald-900 hover:bg-gray-100 rounded-lg transition"
              title="Shopping Cart"
            >
              <ShoppingCart className="w-5 h-5" />
              {totalItems > 0 && (
                <span className="absolute -top-1 -right-1 bg-emerald-700 text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center">
                  {totalItems > 99 ? "99+" : totalItems}
                </span>
              )}
            </Link>

            {user ? (
              <div className="flex items-center gap-3">
                {isCoordinator && (
                  <Link
                    href="/admin"
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-900 text-white hover:bg-emerald-950 transition"
                  >
                    <LayoutDashboard className="w-4 h-4" />
                    Admin Workspace
                  </Link>
                )}

                <Link
                  href="/member"
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-gray-300 text-gray-700 hover:bg-gray-50 transition"
                >
                  <UserIcon className="w-4 h-4 text-emerald-700" />
                  <span>{user.display_name.split(" ")[0]}</span>
                  <Badge variant="neutral" size="sm">
                    {user.primary_role.replace("_", " ")}
                  </Badge>
                </Link>

                <button
                  type="button"
                  onClick={() => logout()}
                  title="Log out"
                  className="p-1.5 text-gray-500 hover:text-gray-700 rounded-lg hover:bg-gray-100 transition"
                >
                  <LogOut className="w-4 h-4" />
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Link
                  href="/login"
                  className="px-3.5 py-2 text-sm font-medium text-gray-700 hover:text-gray-900 hover:bg-gray-50 rounded-lg transition"
                >
                  Sign In
                </Link>
                <Link
                  href="/register"
                  className="px-4 py-2 text-sm font-medium text-white bg-emerald-800 hover:bg-emerald-900 rounded-lg shadow-sm transition"
                >
                  Join Movement
                </Link>
              </div>
            )}
          </div>

          {/* Mobile Menu & Cart Trigger */}
          <div className="flex md:hidden items-center gap-2">
            <Link
              href="/marketplace/cart"
              className="relative p-2 text-gray-700 hover:text-emerald-900 rounded-lg"
              title="Shopping Cart"
            >
              <ShoppingCart className="w-5 h-5" />
              {totalItems > 0 && (
                <span className="absolute -top-1 -right-1 bg-emerald-700 text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center">
                  {totalItems > 99 ? "99+" : totalItems}
                </span>
              )}
            </Link>
            <button
              type="button"
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="p-2 text-gray-600 hover:text-gray-900 focus:outline-none"
              aria-label="Toggle navigation menu"
            >
              {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Drawer */}
      {mobileMenuOpen && (
        <div className="md:hidden border-t border-gray-200 bg-white px-4 pt-2 pb-6 space-y-3">
          <nav className="space-y-1">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setMobileMenuOpen(false)}
                className={`block px-3 py-2.5 rounded-lg text-base font-medium ${
                  isActive(link.href)
                    ? "bg-emerald-50 text-emerald-900 font-semibold"
                    : "text-gray-700 hover:bg-gray-50"
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>

          <div className="pt-3 border-t border-gray-100 space-y-2">
            {user ? (
              <>
                <div className="px-3 py-2 bg-gray-50 rounded-lg text-sm text-gray-700">
                  <div className="font-semibold text-gray-900">{user.display_name}</div>
                  <div className="text-xs text-gray-500">{user.membership_reference}</div>
                  <div className="mt-1">
                    <Badge variant="success" size="sm">{user.primary_role.replace("_", " ")}</Badge>
                  </div>
                </div>

                {isCoordinator && (
                  <Link
                    href="/admin"
                    onClick={() => setMobileMenuOpen(false)}
                    className="block w-full text-center px-4 py-2.5 text-sm font-semibold text-white bg-emerald-900 rounded-lg"
                  >
                    Admin Workspace
                  </Link>
                )}

                <Link
                  href="/member"
                  onClick={() => setMobileMenuOpen(false)}
                  className="block w-full text-center px-4 py-2.5 text-sm font-medium border border-gray-300 text-gray-700 rounded-lg"
                >
                  Member Dashboard
                </Link>

                <button
                  type="button"
                  onClick={() => {
                    logout();
                    setMobileMenuOpen(false);
                  }}
                  className="block w-full text-center px-4 py-2.5 text-sm font-medium text-rose-700 bg-rose-50 rounded-lg"
                >
                  Sign Out
                </button>
              </>
            ) : (
              <div className="grid grid-cols-2 gap-2">
                <Link
                  href="/login"
                  onClick={() => setMobileMenuOpen(false)}
                  className="text-center px-4 py-2.5 text-sm font-medium border border-gray-300 rounded-lg text-gray-700"
                >
                  Sign In
                </Link>
                <Link
                  href="/register"
                  onClick={() => setMobileMenuOpen(false)}
                  className="text-center px-4 py-2.5 text-sm font-medium text-white bg-emerald-800 rounded-lg"
                >
                  Join
                </Link>
              </div>
            )}
          </div>
        </div>
      )}
    </header>
  );
}
