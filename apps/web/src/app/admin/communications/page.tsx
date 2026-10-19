"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Send,
  Mail,
  MessageSquare,
  Server,
  RefreshCw,
  AlertTriangle,
  CheckCircle2,
  Clock,
  RotateCcw,
  Eye,
  Edit3,
  Calendar,
  Shield,
  Layers,
  ArrowLeft,
  X,
  FileText,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import {
  CommunicationsOverviewStats,
  NotificationOutboxItem,
  NotificationDeliveryRecord,
  NotificationTemplate,
  NotificationSchedule,
  ProviderHealthResponse,
} from "@/lib/types";

export default function AdminCommunicationsPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<"queue" | "failed" | "logs" | "schedules" | "templates">("queue");
  const [stats, setStats] = useState<CommunicationsOverviewStats | null>(null);
  const [health, setHealth] = useState<ProviderHealthResponse | null>(null);

  // Queue state
  const [queueItems, setQueueItems] = useState<NotificationOutboxItem[]>([]);
  const [queueTotal, setQueueTotal] = useState(0);
  const [queuePage, setQueuePage] = useState(1);
  const [queueStateFilter, setQueueStateFilter] = useState<string>("all");
  const [queueChannelFilter, setQueueChannelFilter] = useState<string>("all");

  // Failed state
  const [failedItems, setFailedItems] = useState<NotificationOutboxItem[]>([]);
  const [failedTotal, setFailedTotal] = useState(0);
  const [failedPage, setFailedPage] = useState(1);

  // Delivery logs state
  const [deliveryLogs, setDeliveryLogs] = useState<NotificationDeliveryRecord[]>([]);
  const [logsTotal, setLogsTotal] = useState(0);
  const [logsPage, setLogsPage] = useState(1);
  const [logsProviderFilter, setLogsProviderFilter] = useState<string>("all");

  // Schedules state
  const [schedules, setSchedules] = useState<NotificationSchedule[]>([]);
  const [schedulesTotal, setSchedulesTotal] = useState(0);
  const [schedulesPage, setSchedulesPage] = useState(1);

  // Templates state
  const [templates, setTemplates] = useState<NotificationTemplate[]>([]);
  const [editingTemplate, setEditingTemplate] = useState<NotificationTemplate | null>(null);
  const [templateTitle, setTemplateTitle] = useState("");
  const [templateBody, setTemplateBody] = useState("");
  const [templateActive, setTemplateActive] = useState(true);

  const [loading, setLoading] = useState(true);
  const [actionInProgress, setActionInProgress] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (!authLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/communications");
    }
  }, [user, isCoordinator, authLoading, router]);

  const fetchOverview = useCallback(async () => {
    try {
      const [statsRes, healthRes] = await Promise.all([
        api.get<CommunicationsOverviewStats>("/admin/communications/overview"),
        api.get<ProviderHealthResponse>("/admin/communications/provider-health"),
      ]);
      if (statsRes) setStats(statsRes);
      if (healthRes) setHealth(healthRes);
    } catch (err) {
      console.error("Failed to load overview data", err);
    }
  }, []);

  const fetchQueue = useCallback(async () => {
    setLoading(true);
    try {
      const p = new URLSearchParams();
      p.set("page", queuePage.toString());
      p.set("limit", "15");
      if (queueStateFilter !== "all") p.set("delivery_state", queueStateFilter);
      if (queueChannelFilter !== "all") p.set("channel", queueChannelFilter);

      const res = await api.get<{ items: NotificationOutboxItem[]; total: number }>(`/admin/communications/deliveries?${p.toString()}`);
      if (res) {
        setQueueItems(res.items || []);
        setQueueTotal(res.total || 0);
      }
    } catch (err) {
      console.error("Failed to load queue", err);
    } finally {
      setLoading(false);
    }
  }, [queuePage, queueStateFilter, queueChannelFilter]);

  const fetchFailed = useCallback(async () => {
    setLoading(true);
    try {
      const p = new URLSearchParams();
      p.set("page", failedPage.toString());
      p.set("limit", "15");
      const res = await api.get<{ items: NotificationOutboxItem[]; total: number }>(`/admin/communications/failed?${p.toString()}`);
      if (res) {
        setFailedItems(res.items || []);
        setFailedTotal(res.total || 0);
      }
    } catch (err) {
      console.error("Failed to load failed outbox", err);
    } finally {
      setLoading(false);
    }
  }, [failedPage]);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    try {
      const p = new URLSearchParams();
      p.set("page", logsPage.toString());
      p.set("limit", "15");
      if (logsProviderFilter !== "all") p.set("provider", logsProviderFilter);

      const res = await api.get<{ items: NotificationDeliveryRecord[]; total: number }>(`/admin/communications/logs?${p.toString()}`);
      if (res) {
        setDeliveryLogs(res.items || []);
        setLogsTotal(res.total || 0);
      }
    } catch (err) {
      console.error("Failed to load delivery logs", err);
    } finally {
      setLoading(false);
    }
  }, [logsPage, logsProviderFilter]);

  const fetchSchedules = useCallback(async () => {
    setLoading(true);
    try {
      const p = new URLSearchParams();
      p.set("page", schedulesPage.toString());
      p.set("limit", "15");
      const res = await api.get<{ items: NotificationSchedule[]; total: number }>(`/admin/communications/schedules?${p.toString()}`);
      if (res) {
        setSchedules(res.items || []);
        setSchedulesTotal(res.total || 0);
      }
    } catch (err) {
      console.error("Failed to load schedules", err);
    } finally {
      setLoading(false);
    }
  }, [schedulesPage]);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ templates: NotificationTemplate[] }>("/admin/communications/templates");
      if (res && res.templates) {
        setTemplates(res.templates);
      }
    } catch (err) {
      console.error("Failed to load templates", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user && isCoordinator) {
      fetchOverview();
    }
  }, [user, isCoordinator, fetchOverview]);

  useEffect(() => {
    if (!user || !isCoordinator) return;
    if (activeTab === "queue") fetchQueue();
    else if (activeTab === "failed") fetchFailed();
    else if (activeTab === "logs") fetchLogs();
    else if (activeTab === "schedules") fetchSchedules();
    else if (activeTab === "templates") fetchTemplates();
  }, [user, isCoordinator, activeTab, fetchQueue, fetchFailed, fetchLogs, fetchSchedules, fetchTemplates]);

  const handleRetry = async (outboxID: string) => {
    setActionInProgress(outboxID);
    setMessage(null);
    try {
      await api.post(`/admin/communications/deliveries/${outboxID}/retry`, {});
      setMessage({ type: "success", text: `Message ${outboxID.slice(0, 8)} reset to pending for immediate delivery.` });
      fetchOverview();
      if (activeTab === "failed") fetchFailed();
      if (activeTab === "queue") fetchQueue();
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || "Failed to retry message" });
    } finally {
      setActionInProgress(null);
    }
  };

  const handleOpenEditTemplate = (tpl: NotificationTemplate) => {
    setEditingTemplate(tpl);
    setTemplateTitle(tpl.title_template);
    setTemplateBody(tpl.body_template);
    setTemplateActive(tpl.is_active);
  };

  const handleSaveTemplate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingTemplate) return;

    setActionInProgress("template");
    try {
      await api.put(`/admin/communications/templates/${editingTemplate.id}`, {
        title_template: templateTitle,
        body_template: templateBody,
        is_active: templateActive,
      });
      setMessage({ type: "success", text: `Template ${editingTemplate.template_code} updated successfully.` });
      setEditingTemplate(null);
      fetchTemplates();
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || "Failed to update template" });
    } finally {
      setActionInProgress(null);
    }
  };

  const getStatusBadge = (state: string) => {
    switch (state) {
      case "delivered":
      case "accepted":
      case "ready":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-100 text-emerald-800">Delivered</span>;
      case "pending":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-amber-100 text-amber-800">Pending</span>;
      case "processing":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-blue-100 text-blue-800">Processing</span>;
      case "failed":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-rose-100 text-rose-800">Failed</span>;
      case "dead_letter":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-rose-900 text-white">Dead Letter</span>;
      case "skipped":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-gray-100 text-gray-700">Opt-Out Skipped</span>;
      case "sandbox":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-purple-100 text-purple-800">Dev Sandbox</span>;
      case "unconfigured":
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-yellow-100 text-yellow-800">Unconfigured</span>;
      default:
        return <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-gray-100 text-gray-800">{state}</span>;
    }
  };

  if (authLoading || !user || !isCoordinator) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-sm text-gray-500">Checking coordinator authorization...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white p-6 rounded-2xl shadow-sm border border-gray-200">
          <div className="flex items-center gap-3">
            <Link
              href="/admin"
              className="p-2 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition"
              title="Back to Admin Dashboard"
            >
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <div>
              <h1 className="text-xl sm:text-2xl font-bold text-gray-900">Communications & Delivery Operations</h1>
              <p className="text-xs sm:text-sm text-gray-500">
                Transactional outbox queue, Brevo email, Termii SMS, and automated reminder schedules.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => {
                fetchOverview();
                if (activeTab === "queue") fetchQueue();
                if (activeTab === "failed") fetchFailed();
                if (activeTab === "logs") fetchLogs();
                if (activeTab === "schedules") fetchSchedules();
                if (activeTab === "templates") fetchTemplates();
              }}
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl border border-gray-300 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              Refresh
            </button>
          </div>
        </div>

        {message && (
          <div
            className={`p-4 rounded-xl text-sm flex items-center gap-3 ${
              message.type === "success"
                ? "bg-emerald-50 text-emerald-900 border border-emerald-200"
                : "bg-rose-50 text-rose-900 border border-rose-200"
            }`}
          >
            {message.type === "success" ? <CheckCircle2 className="w-5 h-5 flex-shrink-0" /> : <AlertTriangle className="w-5 h-5 flex-shrink-0" />}
            <span>{message.text}</span>
          </div>
        )}

        {/* Provider Health Row */}
        {health && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Brevo */}
            <div className="bg-white rounded-xl p-4 shadow-sm border border-gray-200 flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                  <Mail className="w-4 h-4 text-blue-600" />
                  Brevo Email
                </div>
                <div className="mt-2 flex items-center gap-2">
                  {getStatusBadge(health.brevo_provider.status)}
                  <span className="text-xs text-gray-500">{health.brevo_provider.provider}</span>
                </div>
              </div>
              <div className="text-[11px] text-gray-400">SMTP v3</div>
            </div>

            {/* Termii */}
            <div className="bg-white rounded-xl p-4 shadow-sm border border-gray-200 flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                  <MessageSquare className="w-4 h-4 text-amber-600" />
                  Termii SMS
                </div>
                <div className="mt-2 flex items-center gap-2">
                  {getStatusBadge(health.termii_provider.status)}
                  <span className="text-xs text-gray-500">{health.termii_provider.provider}</span>
                </div>
              </div>
              <div className="text-[11px] text-gray-400">NG Gateway</div>
            </div>

            {/* Outbox Worker */}
            <div className="bg-white rounded-xl p-4 shadow-sm border border-gray-200 flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                  <Server className="w-4 h-4 text-emerald-600" />
                  Outbox Worker
                </div>
                <div className="mt-2 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-100 text-emerald-800">Active</span>
                  <span className="text-xs text-gray-500">Lagos Time: {new Date(health.lagos_time).toLocaleTimeString("en-NG", { hour: "2-digit", minute: "2-digit" })}</span>
                </div>
              </div>
              <div className="text-[11px] text-gray-400">WAT (UTC+1)</div>
            </div>
          </div>
        )}

        {/* Overview Stats */}
        {stats && (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-gray-500 block">Total Queued</span>
              <span className="text-xl font-bold text-gray-900 mt-1 block">{stats.total_count}</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-amber-600 font-medium block">Pending</span>
              <span className="text-xl font-bold text-amber-700 mt-1 block">{stats.pending_count}</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-blue-600 font-medium block">Processing</span>
              <span className="text-xl font-bold text-blue-700 mt-1 block">{stats.processing_count}</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-emerald-600 font-medium block">Delivered</span>
              <span className="text-xl font-bold text-emerald-700 mt-1 block">{stats.delivered_count}</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-rose-600 font-medium block">Failed / Dead-Letter</span>
              <span className="text-xl font-bold text-rose-700 mt-1 block">{stats.failed_count}</span>
            </div>
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm">
              <span className="text-xs text-gray-500 block">Opt-Out Skipped</span>
              <span className="text-xl font-bold text-gray-600 mt-1 block">{stats.skipped_count}</span>
            </div>
          </div>
        )}

        {/* Main Workspace Navigation */}
        <div className="bg-white rounded-2xl shadow-sm border border-gray-200 overflow-hidden">
          <div className="flex border-b border-gray-200 overflow-x-auto text-sm font-medium">
            <button
              type="button"
              onClick={() => setActiveTab("queue")}
              className={`px-5 py-3.5 border-b-2 whitespace-nowrap transition ${
                activeTab === "queue"
                  ? "border-emerald-800 text-emerald-900 font-bold bg-emerald-50/30"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              }`}
            >
              Delivery Queue ({queueTotal})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("failed")}
              className={`px-5 py-3.5 border-b-2 whitespace-nowrap transition ${
                activeTab === "failed"
                  ? "border-rose-700 text-rose-900 font-bold bg-rose-50/30"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              }`}
            >
              Failed Deliveries ({failedTotal})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("logs")}
              className={`px-5 py-3.5 border-b-2 whitespace-nowrap transition ${
                activeTab === "logs"
                  ? "border-emerald-800 text-emerald-900 font-bold bg-emerald-50/30"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              }`}
            >
              Delivery Audit Logs ({logsTotal})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("schedules")}
              className={`px-5 py-3.5 border-b-2 whitespace-nowrap transition ${
                activeTab === "schedules"
                  ? "border-emerald-800 text-emerald-900 font-bold bg-emerald-50/30"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              }`}
            >
              Reminder Schedules ({schedulesTotal})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("templates")}
              className={`px-5 py-3.5 border-b-2 whitespace-nowrap transition ${
                activeTab === "templates"
                  ? "border-emerald-800 text-emerald-900 font-bold bg-emerald-50/30"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50"
              }`}
            >
              Message Templates ({templates.length})
            </button>
          </div>

          <div className="p-6">
            {/* Tab 1: Delivery Queue */}
            {activeTab === "queue" && (
              <div className="space-y-4">
                <div className="flex flex-wrap items-center justify-between gap-3 text-xs">
                  <div className="flex items-center gap-2">
                    <select
                      value={queueStateFilter}
                      onChange={(e) => {
                        setQueueStateFilter(e.target.value);
                        setQueuePage(1);
                      }}
                      className="bg-gray-50 border border-gray-300 rounded-lg px-2.5 py-1.5 text-gray-700"
                    >
                      <option value="all">All States</option>
                      <option value="pending">Pending</option>
                      <option value="processing">Processing</option>
                      <option value="delivered">Delivered</option>
                      <option value="failed">Failed</option>
                      <option value="dead_letter">Dead Letter</option>
                      <option value="skipped">Skipped</option>
                    </select>

                    <select
                      value={queueChannelFilter}
                      onChange={(e) => {
                        setQueueChannelFilter(e.target.value);
                        setQueuePage(1);
                      }}
                      className="bg-gray-50 border border-gray-300 rounded-lg px-2.5 py-1.5 text-gray-700"
                    >
                      <option value="all">All Channels</option>
                      <option value="email">Email</option>
                      <option value="sms">SMS</option>
                      <option value="in_app">In-App</option>
                    </select>
                  </div>
                  <span className="text-gray-400">Showing page {queuePage}</span>
                </div>

                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-semibold">
                      <tr>
                        <th className="py-3 px-4">Event Type</th>
                        <th className="py-3 px-4">Channel</th>
                        <th className="py-3 px-4">Recipient</th>
                        <th className="py-3 px-4">State</th>
                        <th className="py-3 px-4">Attempts</th>
                        <th className="py-3 px-4">Next Attempt</th>
                        <th className="py-3 px-4">Created</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {loading ? (
                        <tr>
                          <td colSpan={7} className="py-8 text-center text-gray-400">Loading queue...</td>
                        </tr>
                      ) : queueItems.length === 0 ? (
                        <tr>
                          <td colSpan={7} className="py-8 text-center text-gray-400">No items found in queue.</td>
                        </tr>
                      ) : (
                        queueItems.map((item) => (
                          <tr key={item.id} className="hover:bg-gray-50/50">
                            <td className="py-3 px-4 font-semibold text-gray-900">{item.event_type}</td>
                            <td className="py-3 px-4 uppercase font-bold text-gray-600">{item.channel}</td>
                            <td className="py-3 px-4 text-gray-600">
                              {item.recipient_name || "—"}{" "}
                              <span className="text-gray-400 block text-[11px]">
                                {item.recipient_email || item.recipient_phone || "—"}
                              </span>
                            </td>
                            <td className="py-3 px-4">{getStatusBadge(item.delivery_state)}</td>
                            <td className="py-3 px-4 text-gray-600">
                              {item.attempt_count} / {item.max_attempts}
                            </td>
                            <td className="py-3 px-4 text-gray-500 whitespace-nowrap">
                              {new Date(item.next_attempt_at).toLocaleTimeString("en-NG", { hour: "2-digit", minute: "2-digit" })}
                            </td>
                            <td className="py-3 px-4 text-gray-400 whitespace-nowrap">
                              {new Date(item.created_at).toLocaleDateString("en-NG", { month: "short", day: "numeric" })}
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Tab 2: Failed Deliveries & Retries */}
            {activeTab === "failed" && (
              <div className="space-y-4">
                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-semibold">
                      <tr>
                        <th className="py-3 px-4">Event & Channel</th>
                        <th className="py-3 px-4">Recipient</th>
                        <th className="py-3 px-4">State</th>
                        <th className="py-3 px-4">Error Category</th>
                        <th className="py-3 px-4">Last Error</th>
                        <th className="py-3 px-4">Attempts</th>
                        <th className="py-3 px-4 text-right">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {loading ? (
                        <tr>
                          <td colSpan={7} className="py-8 text-center text-gray-400">Loading failed messages...</td>
                        </tr>
                      ) : failedItems.length === 0 ? (
                        <tr>
                          <td colSpan={7} className="py-8 text-center text-emerald-800 bg-emerald-50/20 font-medium">
                            No failed or dead-letter messages in outbox. All clear!
                          </td>
                        </tr>
                      ) : (
                        failedItems.map((item) => (
                          <tr key={item.id} className="hover:bg-gray-50/50">
                            <td className="py-3 px-4">
                              <span className="font-semibold text-gray-900 block">{item.event_type}</span>
                              <span className="text-[10px] uppercase font-bold text-gray-500">{item.channel}</span>
                            </td>
                            <td className="py-3 px-4 text-gray-700">
                              {item.recipient_name || "—"}
                              <span className="text-gray-400 block text-[11px]">
                                {item.recipient_email || item.recipient_phone || "—"}
                              </span>
                            </td>
                            <td className="py-3 px-4">{getStatusBadge(item.delivery_state)}</td>
                            <td className="py-3 px-4">
                              <span className="px-2 py-0.5 rounded bg-gray-100 text-gray-700 font-mono text-[10px]">
                                {item.last_error_category || "error"}
                              </span>
                            </td>
                            <td className="py-3 px-4 text-rose-700 max-w-xs truncate" title={item.last_error}>
                              {item.last_error || "Unknown error"}
                            </td>
                            <td className="py-3 px-4 text-gray-600">
                              {item.attempt_count} / {item.max_attempts}
                            </td>
                            <td className="py-3 px-4 text-right">
                              <button
                                type="button"
                                onClick={() => handleRetry(item.id)}
                                disabled={actionInProgress === item.id}
                                className="inline-flex items-center gap-1 px-3 py-1 bg-emerald-800 text-white rounded-lg text-xs font-semibold hover:bg-emerald-900 disabled:opacity-50 transition"
                              >
                                <RotateCcw className="w-3 h-3" />
                                {actionInProgress === item.id ? "Retrying..." : "Retry"}
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Tab 3: Delivery Audit History */}
            {activeTab === "logs" && (
              <div className="space-y-4">
                <div className="flex items-center gap-2">
                  <select
                    value={logsProviderFilter}
                    onChange={(e) => {
                      setLogsProviderFilter(e.target.value);
                      setLogsPage(1);
                    }}
                    className="bg-gray-50 border border-gray-300 rounded-lg px-2.5 py-1.5 text-xs text-gray-700"
                  >
                    <option value="all">All Providers</option>
                    <option value="brevo">Brevo</option>
                    <option value="termii">Termii</option>
                    <option value="dev_sandbox">Dev Sandbox</option>
                  </select>
                </div>

                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-semibold">
                      <tr>
                        <th className="py-3 px-4">Provider</th>
                        <th className="py-3 px-4">Channel</th>
                        <th className="py-3 px-4">Message Ref</th>
                        <th className="py-3 px-4">Status</th>
                        <th className="py-3 px-4">Attempt</th>
                        <th className="py-3 px-4">Timestamp</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {loading ? (
                        <tr>
                          <td colSpan={6} className="py-8 text-center text-gray-400">Loading logs...</td>
                        </tr>
                      ) : deliveryLogs.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="py-8 text-center text-gray-400">No delivery logs recorded yet.</td>
                        </tr>
                      ) : (
                        deliveryLogs.map((log) => (
                          <tr key={log.id} className="hover:bg-gray-50/50">
                            <td className="py-3 px-4 font-semibold text-gray-900">{log.provider}</td>
                            <td className="py-3 px-4 uppercase font-bold text-gray-600">{log.channel}</td>
                            <td className="py-3 px-4 font-mono text-[11px] text-gray-600">{log.provider_message_ref || "—"}</td>
                            <td className="py-3 px-4">{getStatusBadge(log.status)}</td>
                            <td className="py-3 px-4 text-gray-600">#{log.attempt_number}</td>
                            <td className="py-3 px-4 text-gray-400 whitespace-nowrap">
                              {new Date(log.created_at).toLocaleString("en-NG", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" })}
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Tab 4: Automated Reminder Schedules */}
            {activeTab === "schedules" && (
              <div className="space-y-4">
                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-semibold">
                      <tr>
                        <th className="py-3 px-4">Schedule Type</th>
                        <th className="py-3 px-4">Event Title</th>
                        <th className="py-3 px-4">Event Date</th>
                        <th className="py-3 px-4">Target Dispatch (WAT)</th>
                        <th className="py-3 px-4">Status</th>
                        <th className="py-3 px-4">Dispatched At</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {loading ? (
                        <tr>
                          <td colSpan={6} className="py-8 text-center text-gray-400">Loading schedules...</td>
                        </tr>
                      ) : schedules.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="py-8 text-center text-gray-400">No reminder schedules found.</td>
                        </tr>
                      ) : (
                        schedules.map((sched) => (
                          <tr key={sched.id} className="hover:bg-gray-50/50">
                            <td className="py-3 px-4 font-bold text-gray-900">
                              {sched.schedule_type === "cleanup_reminder_7d" ? "7 Days Before" : "1 Day Before"}
                            </td>
                            <td className="py-3 px-4 font-medium text-emerald-900">{sched.event_title || sched.event_id}</td>
                            <td className="py-3 px-4 text-gray-600">
                              {sched.event_start_time ? new Date(sched.event_start_time).toLocaleDateString("en-NG", { month: "short", day: "numeric" }) : "—"}
                            </td>
                            <td className="py-3 px-4 text-gray-700 whitespace-nowrap">
                              {new Date(sched.target_time).toLocaleDateString("en-NG", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                            </td>
                            <td className="py-3 px-4">{getStatusBadge(sched.status)}</td>
                            <td className="py-3 px-4 text-gray-400">
                              {sched.dispatched_at ? new Date(sched.dispatched_at).toLocaleTimeString("en-NG", { hour: "2-digit", minute: "2-digit" }) : "—"}
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Tab 5: Templates */}
            {activeTab === "templates" && (
              <div className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {templates.map((tpl) => (
                    <div key={tpl.id} className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-2 text-xs">
                      <div className="flex items-center justify-between">
                        <span className="font-bold text-gray-900 text-sm">{tpl.template_code}</span>
                        <div className="flex items-center gap-2">
                          <span className="uppercase text-[10px] font-bold text-gray-500">{tpl.channel}</span>
                          <span className="text-[10px] text-gray-400">v{tpl.version}</span>
                        </div>
                      </div>
                      <p className="font-semibold text-gray-700">Subject: {tpl.title_template}</p>
                      <p className="text-gray-600 line-clamp-3 bg-white p-2.5 rounded-lg border border-gray-100 font-mono text-[11px]">
                        {tpl.body_template}
                      </p>
                      <div className="flex items-center justify-between pt-2 border-t border-gray-200">
                        <span className={`text-[10px] font-semibold ${tpl.is_active ? "text-emerald-700" : "text-gray-400"}`}>
                          {tpl.is_active ? "Active" : "Disabled"}
                        </span>
                        <button
                          type="button"
                          onClick={() => handleOpenEditTemplate(tpl)}
                          className="inline-flex items-center gap-1 text-emerald-800 hover:text-emerald-950 font-semibold"
                        >
                          <Edit3 className="w-3.5 h-3.5" />
                          Edit Template
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Edit Template Modal */}
        {editingTemplate && (
          <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
            <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-5 shadow-2xl animate-in fade-in zoom-in-95">
              <div className="flex items-center justify-between border-b border-gray-200 pb-3">
                <div>
                  <h3 className="font-bold text-gray-900 text-base">Edit Template: {editingTemplate.template_code}</h3>
                  <span className="text-xs text-gray-500 uppercase">{editingTemplate.channel} • Version {editingTemplate.version}</span>
                </div>
                <button
                  type="button"
                  onClick={() => setEditingTemplate(null)}
                  className="p-1 rounded-lg text-gray-400 hover:text-gray-600"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <form onSubmit={handleSaveTemplate} className="space-y-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Title / Subject Template</label>
                  <input
                    type="text"
                    value={templateTitle}
                    onChange={(e) => setTemplateTitle(e.target.value)}
                    required
                    className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-1 focus:ring-emerald-700"
                  />
                  <p className="text-[10px] text-gray-400 mt-1">Available placeholders: {"{{.EventTitle}}"}, {"{{.RecipientName}}"}</p>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Body Template</label>
                  <textarea
                    rows={5}
                    value={templateBody}
                    onChange={(e) => setTemplateBody(e.target.value)}
                    required
                    className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg font-mono focus:ring-1 focus:ring-emerald-700"
                  />
                  <p className="text-[10px] text-gray-400 mt-1">Placeholders: {"{{.EventTitle}}"}, {"{{.EventDate}}"}, {"{{.EventLocation}}"}, {"{{.StartTime}}"}</p>
                </div>

                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="template_active"
                    checked={templateActive}
                    onChange={(e) => setTemplateActive(e.target.checked)}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300"
                  />
                  <label htmlFor="template_active" className="text-xs font-medium text-gray-700">
                    Template is active and available for system dispatch
                  </label>
                </div>

                <div className="flex items-center justify-end gap-2 pt-3 border-t border-gray-200">
                  <button
                    type="button"
                    onClick={() => setEditingTemplate(null)}
                    className="px-4 py-2 rounded-lg border border-gray-300 text-xs font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={actionInProgress === "template"}
                    className="px-5 py-2 rounded-lg bg-emerald-800 text-white text-xs font-bold hover:bg-emerald-900 disabled:opacity-50"
                  >
                    {actionInProgress === "template" ? "Saving..." : "Save Template"}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
