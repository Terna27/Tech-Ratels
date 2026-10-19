"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Bell,
  Check,
  CheckCheck,
  Calendar,
  Shield,
  Award,
  Camera,
  ShoppingBag,
  Briefcase,
  AlertCircle,
  ExternalLink,
  Settings,
} from "lucide-react";
import { api } from "@/lib/api";
import { InAppNotification } from "@/lib/types";

export function NotificationBell() {
  const router = useRouter();
  const [unreadCount, setUnreadCount] = useState<number>(0);
  const [recentNotifications, setRecentNotifications] = useState<InAppNotification[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const fetchUnreadCount = async () => {
    try {
      const res = await api.get<{ unread_count: number }>("/me/notifications/unread-count");
      if (res && typeof res.unread_count === "number") {
        setUnreadCount(res.unread_count);
      }
    } catch {
      // Ignore background unread check failure
    }
  };

  const fetchRecent = async () => {
    setLoading(true);
    try {
      const res = await api.get<{ items: InAppNotification[]; unread_count: number }>("/me/notifications?limit=5");
      if (res && res.items) {
        setRecentNotifications(res.items);
        if (typeof res.unread_count === "number") {
          setUnreadCount(res.unread_count);
        }
      }
    } catch (err) {
      console.error("Failed to load notifications", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUnreadCount();
    const interval = setInterval(fetchUnreadCount, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleToggle = () => {
    if (!isOpen) {
      fetchRecent();
    }
    setIsOpen(!isOpen);
  };

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleMarkAsRead = async (id: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    try {
      await api.post(`/me/notifications/${id}/read`, {});
      setRecentNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (err) {
      console.error("Failed to mark as read", err);
    }
  };

  const handleMarkAllAsRead = async () => {
    try {
      await api.post("/me/notifications/read-all", {});
      setRecentNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnreadCount(0);
    } catch (err) {
      console.error("Failed to mark all as read", err);
    }
  };

  const handleNotificationClick = async (notif: InAppNotification) => {
    if (!notif.is_read) {
      await handleMarkAsRead(notif.id);
    }
    setIsOpen(false);
    if (notif.action_url) {
      // If internal or starts with slash or domain
      const url = notif.action_url;
      if (url.startsWith("/") || url.includes(window.location.host)) {
        const path = url.replace(/https?:\/\/[^/]+/, "");
        router.push(path || "/");
      } else {
        window.open(url, "_blank");
      }
    } else {
      router.push("/notifications");
    }
  };

  const getNotificationIcon = (type: string) => {
    if (type.includes("cleanup") || type.includes("event")) {
      return <Calendar className="w-4 h-4 text-emerald-600" />;
    }
    if (type.includes("leadership")) {
      return <Shield className="w-4 h-4 text-amber-600" />;
    }
    if (type.includes("claim") || type.includes("participation")) {
      return <Award className="w-4 h-4 text-blue-600" />;
    }
    if (type.includes("media")) {
      return <Camera className="w-4 h-4 text-purple-600" />;
    }
    if (type.includes("order")) {
      return <ShoppingBag className="w-4 h-4 text-emerald-600" />;
    }
    if (type.includes("career")) {
      return <Briefcase className="w-4 h-4 text-indigo-600" />;
    }
    return <AlertCircle className="w-4 h-4 text-gray-500" />;
  };

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        type="button"
        onClick={handleToggle}
        className="relative p-2 text-gray-700 hover:text-emerald-900 hover:bg-gray-100 rounded-lg transition"
        title="Notifications"
        aria-label="Notifications"
      >
        <Bell className="w-5 h-5" />
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 bg-red-600 text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center animate-pulse">
            {unreadCount > 9 ? "9+" : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 sm:w-96 bg-white rounded-xl shadow-xl border border-gray-200 z-50 overflow-hidden animate-in fade-in slide-in-from-top-2 duration-150">
          {/* Header */}
          <div className="px-4 py-3 bg-gray-50 border-b border-gray-100 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-gray-900 text-sm">Notifications</span>
              {unreadCount > 0 && (
                <span className="bg-emerald-100 text-emerald-800 text-xs px-2 py-0.5 rounded-full font-medium">
                  {unreadCount} unread
                </span>
              )}
            </div>
            {unreadCount > 0 && (
              <button
                type="button"
                onClick={handleMarkAllAsRead}
                className="text-xs text-emerald-700 hover:text-emerald-900 flex items-center gap-1 font-medium transition"
              >
                <CheckCheck className="w-3.5 h-3.5" />
                Mark all read
              </button>
            )}
          </div>

          {/* List */}
          <div className="max-h-80 overflow-y-auto divide-y divide-gray-100">
            {loading ? (
              <div className="py-8 text-center text-gray-400 text-xs">Loading notifications...</div>
            ) : recentNotifications.length === 0 ? (
              <div className="py-8 text-center px-4">
                <Bell className="w-8 h-8 text-gray-300 mx-auto mb-2" />
                <p className="text-sm font-medium text-gray-700">No notifications yet</p>
                <p className="text-xs text-gray-500 mt-0.5">
                  We'll alert you about cleanups, leadership appointments, and updates.
                </p>
              </div>
            ) : (
              recentNotifications.map((notif) => (
                <div
                  key={notif.id}
                  onClick={() => handleNotificationClick(notif)}
                  className={`p-3.5 flex items-start gap-3 hover:bg-gray-50 cursor-pointer transition text-left ${
                    !notif.is_read ? "bg-emerald-50/40" : ""
                  }`}
                >
                  <div className="mt-0.5 p-1.5 bg-gray-100 rounded-lg flex-shrink-0">
                    {getNotificationIcon(notif.notification_type)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-1">
                      <p className={`text-xs ${!notif.is_read ? "font-bold text-gray-900" : "font-medium text-gray-800"} truncate`}>
                        {notif.title}
                      </p>
                      {!notif.is_read && (
                        <span className="w-2 h-2 rounded-full bg-emerald-600 flex-shrink-0" />
                      )}
                    </div>
                    <p className="text-xs text-gray-600 mt-0.5 line-clamp-2 leading-relaxed">
                      {notif.body}
                    </p>
                    <div className="flex items-center justify-between mt-1.5 text-[10px] text-gray-400">
                      <span>{new Date(notif.created_at).toLocaleDateString("en-NG", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}</span>
                      {!notif.is_read && (
                        <button
                          type="button"
                          onClick={(e) => handleMarkAsRead(notif.id, e)}
                          className="text-emerald-700 hover:text-emerald-900 font-medium flex items-center gap-0.5"
                          title="Mark read"
                        >
                          <Check className="w-3 h-3" />
                          Read
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Footer */}
          <div className="px-4 py-2.5 bg-gray-50 border-t border-gray-100 flex items-center justify-between text-xs">
            <Link
              href="/notifications"
              onClick={() => setIsOpen(false)}
              className="text-emerald-800 hover:text-emerald-950 font-semibold flex items-center gap-1 transition"
            >
              View all in inbox
              <ExternalLink className="w-3 h-3" />
            </Link>
            <Link
              href="/settings/notifications"
              onClick={() => setIsOpen(false)}
              className="text-gray-500 hover:text-gray-700 flex items-center gap-1 transition"
              title="Preferences"
            >
              <Settings className="w-3.5 h-3.5" />
              Settings
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
