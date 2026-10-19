"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Bell,
  Check,
  CheckCheck,
  Trash2,
  Calendar,
  Shield,
  Award,
  Camera,
  ShoppingBag,
  Briefcase,
  AlertCircle,
  ExternalLink,
  Settings,
  Filter,
  RefreshCw,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { InAppNotification, PaginatedNotificationsResponse } from "@/lib/types";

export default function NotificationsPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [notifications, setNotifications] = useState<InAppNotification[]>([]);
  const [total, setTotal] = useState(0);
  const [unreadCount, setUnreadCount] = useState(0);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [selectedType, setSelectedType] = useState<string>("all");
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);

  const fetchNotifications = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("page", page.toString());
      params.set("limit", "15");
      if (unreadOnly) {
        params.set("unread_only", "true");
      }
      if (selectedType !== "all") {
        params.set("type", selectedType);
      }

      const res = await api.get<PaginatedNotificationsResponse>(`/me/notifications?${params.toString()}`);
      if (res) {
        setNotifications(res.items || []);
        setTotal(res.total || 0);
        setUnreadCount(res.unread_count || 0);
        setTotalPages(res.total_pages || 1);
      }
    } catch (err) {
      console.error("Failed to load notifications", err);
    } finally {
      setLoading(false);
    }
  }, [user, page, unreadOnly, selectedType]);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/notifications");
    }
  }, [user, authLoading, router]);

  useEffect(() => {
    if (user) {
      fetchNotifications();
    }
  }, [user, fetchNotifications]);

  const handleMarkAsRead = async (id: string) => {
    try {
      await api.post(`/me/notifications/${id}/read`, {});
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true, read_at: new Date().toISOString() } : n))
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (err) {
      console.error("Failed to mark as read", err);
    }
  };

  const handleMarkAllAsRead = async () => {
    setActionLoading(true);
    try {
      await api.post("/me/notifications/read-all", {});
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnreadCount(0);
    } catch (err) {
      console.error("Failed to mark all as read", err);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/me/notifications/${id}`);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
      setTotal((prev) => Math.max(0, prev - 1));
    } catch (err) {
      console.error("Failed to delete notification", err);
    }
  };

  const handleOpenAction = async (notif: InAppNotification) => {
    if (!notif.is_read) {
      await handleMarkAsRead(notif.id);
    }
    if (notif.action_url) {
      const url = notif.action_url;
      if (url.startsWith("/") || url.includes(window.location.host)) {
        const path = url.replace(/https?:\/\/[^/]+/, "");
        router.push(path || "/");
      } else {
        window.open(url, "_blank");
      }
    }
  };

  const getNotificationIcon = (type: string) => {
    if (type.includes("cleanup") || type.includes("event")) {
      return <Calendar className="w-5 h-5 text-emerald-600" />;
    }
    if (type.includes("leadership")) {
      return <Shield className="w-5 h-5 text-amber-600" />;
    }
    if (type.includes("claim") || type.includes("participation")) {
      return <Award className="w-5 h-5 text-blue-600" />;
    }
    if (type.includes("media")) {
      return <Camera className="w-5 h-5 text-purple-600" />;
    }
    if (type.includes("order")) {
      return <ShoppingBag className="w-5 h-5 text-emerald-600" />;
    }
    if (type.includes("career")) {
      return <Briefcase className="w-5 h-5 text-indigo-600" />;
    }
    return <AlertCircle className="w-5 h-5 text-gray-500" />;
  };

  if (authLoading || !user) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-sm text-gray-500">Authenticating...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-4xl mx-auto space-y-6">
        {/* Page Header */}
        <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <div className="p-2.5 bg-emerald-100 rounded-xl text-emerald-900">
                <Bell className="w-6 h-6" />
              </div>
              <div>
                <h1 className="text-xl sm:text-2xl font-bold text-gray-900">Notifications Inbox</h1>
                <p className="text-xs sm:text-sm text-gray-500">
                  Stay updated on cleanups, verified claims, announcements, and leadership alerts.
                </p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 self-start sm:self-auto">
            {unreadCount > 0 && (
              <button
                type="button"
                onClick={handleMarkAllAsRead}
                disabled={actionLoading}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-50 text-emerald-800 hover:bg-emerald-100 border border-emerald-200 transition"
              >
                <CheckCheck className="w-3.5 h-3.5" />
                Mark all read
              </button>
            )}

            <button
              type="button"
              onClick={() => fetchNotifications()}
              className="p-2 rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-100 transition"
              title="Refresh"
            >
              <RefreshCw className="w-4 h-4" />
            </button>

            <Link
              href="/settings/notifications"
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-gray-300 text-gray-700 hover:bg-gray-50 transition"
            >
              <Settings className="w-3.5 h-3.5 text-gray-500" />
              Preferences
            </Link>
          </div>
        </div>

        {/* Filters Bar */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-3 sm:p-4 flex flex-wrap items-center justify-between gap-3 text-xs sm:text-sm">
          <div className="flex items-center gap-1 bg-gray-100 p-1 rounded-lg">
            <button
              type="button"
              onClick={() => {
                setUnreadOnly(false);
                setPage(1);
              }}
              className={`px-3 py-1.5 rounded-md font-medium transition ${
                !unreadOnly ? "bg-white text-gray-900 shadow-xs" : "text-gray-600 hover:text-gray-900"
              }`}
            >
              All ({total})
            </button>
            <button
              type="button"
              onClick={() => {
                setUnreadOnly(true);
                setPage(1);
              }}
              className={`px-3 py-1.5 rounded-md font-medium transition ${
                unreadOnly ? "bg-white text-emerald-800 font-semibold shadow-xs" : "text-gray-600 hover:text-gray-900"
              }`}
            >
              Unread ({unreadCount})
            </button>
          </div>

          <div className="flex items-center gap-2">
            <Filter className="w-3.5 h-3.5 text-gray-400" />
            <select
              value={selectedType}
              onChange={(e) => {
                setSelectedType(e.target.value);
                setPage(1);
              }}
              className="bg-gray-50 border border-gray-300 rounded-lg px-2.5 py-1.5 text-xs text-gray-700 focus:outline-none focus:ring-1 focus:ring-emerald-700"
            >
              <option value="all">All Notification Types</option>
              <option value="cleanup_reminder_7d">Cleanup Reminders (7 Days)</option>
              <option value="cleanup_reminder_1d">Cleanup Reminders (Tomorrow)</option>
              <option value="event_rescheduled">Schedule Updates</option>
              <option value="claim_approved">Verified Cleanups</option>
              <option value="claim_rejected">Claim Rejections</option>
              <option value="leadership_assigned">Leadership Appointments</option>
              <option value="media_approved">Approved Media</option>
              <option value="order_status">Marketplace Orders</option>
              <option value="career_status">Careers & Jobs</option>
              <option value="project_status">Community Projects</option>
              <option value="finance_action">Financial Notices</option>
            </select>
          </div>
        </div>

        {/* Notifications List */}
        <div className="bg-white rounded-2xl shadow-sm border border-gray-200 overflow-hidden divide-y divide-gray-100">
          {loading ? (
            <div className="py-16 text-center text-gray-400 text-sm">
              <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2 text-emerald-700" />
              Loading your notifications...
            </div>
          ) : notifications.length === 0 ? (
            <div className="py-16 text-center px-4">
              <div className="w-12 h-12 bg-gray-100 rounded-full flex items-center justify-center mx-auto mb-3">
                <Bell className="w-6 h-6 text-gray-400" />
              </div>
              <h3 className="text-base font-semibold text-gray-900">No notifications found</h3>
              <p className="text-xs text-gray-500 max-w-sm mx-auto mt-1">
                {unreadOnly
                  ? "You have no unread notifications right now. Great job catching up!"
                  : "When activities happen in your state or squad, you will see notifications here."}
              </p>
              {unreadOnly && (
                <button
                  type="button"
                  onClick={() => setUnreadOnly(false)}
                  className="mt-4 inline-flex items-center px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-800 text-white hover:bg-emerald-900 transition"
                >
                  View All Notifications
                </button>
              )}
            </div>
          ) : (
            notifications.map((notif) => (
              <div
                key={notif.id}
                className={`p-4 sm:p-5 flex items-start gap-4 transition hover:bg-gray-50 ${
                  !notif.is_read ? "bg-emerald-50/30" : ""
                }`}
              >
                <div className="mt-1 p-2 bg-gray-100 rounded-xl flex-shrink-0">
                  {getNotificationIcon(notif.notification_type)}
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <h4
                        onClick={() => handleOpenAction(notif)}
                        className={`text-sm ${
                          !notif.is_read ? "font-bold text-gray-900" : "font-semibold text-gray-800"
                        } hover:text-emerald-900 cursor-pointer`}
                      >
                        {notif.title}
                      </h4>
                      {!notif.is_read && (
                        <span className="w-2 h-2 rounded-full bg-emerald-600 flex-shrink-0" />
                      )}
                    </div>

                    <span className="text-[11px] text-gray-400 whitespace-nowrap">
                      {new Date(notif.created_at).toLocaleDateString("en-NG", {
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>

                  <p className="text-xs sm:text-sm text-gray-600 mt-1 leading-relaxed">
                    {notif.body}
                  </p>

                  <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      {notif.related_resource_type && (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium bg-gray-100 text-gray-600 uppercase tracking-wide">
                          {notif.related_resource_type}
                        </span>
                      )}

                      {notif.action_url && (
                        <button
                          type="button"
                          onClick={() => handleOpenAction(notif)}
                          className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-800 hover:text-emerald-950 transition"
                        >
                          View Details
                          <ExternalLink className="w-3 h-3" />
                        </button>
                      )}
                    </div>

                    <div className="flex items-center gap-2">
                      {!notif.is_read && (
                        <button
                          type="button"
                          onClick={() => handleMarkAsRead(notif.id)}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded text-xs text-emerald-800 hover:bg-emerald-100 font-medium transition"
                          title="Mark as read"
                        >
                          <Check className="w-3.5 h-3.5" />
                          Mark read
                        </button>
                      )}

                      <button
                        type="button"
                        onClick={() => handleDelete(notif.id)}
                        className="p-1 text-gray-400 hover:text-rose-600 rounded transition"
                        title="Delete notification"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between bg-white px-4 py-3 rounded-xl shadow-sm border border-gray-200">
            <span className="text-xs text-gray-500">
              Page {page} of {totalPages} ({total} total)
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="px-3 py-1 rounded border border-gray-300 text-xs text-gray-700 disabled:opacity-50 hover:bg-gray-50"
              >
                Previous
              </button>
              <button
                type="button"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="px-3 py-1 rounded border border-gray-300 text-xs text-gray-700 disabled:opacity-50 hover:bg-gray-50"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
