"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  Lock,
  Eye,
  EyeOff,
  ShieldAlert,
  Send,
  MessageSquare,
  Clock,
  CheckCircle2,
  AlertTriangle,
  ArrowLeft,
  LifeBuoy,
  FileText,
  User,
  Shield,
  LogOut,
  ExternalLink,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { SafeguardingStatusResponse, SafeguardingMessageItem } from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";

function StatusContent() {
  const searchParams = useSearchParams();
  const urlRef = searchParams.get("ref") || "";
  const urlSec = searchParams.get("sec") || "";

  const [referenceNumber, setReferenceNumber] = useState(urlRef);
  const [followUpSecret, setFollowUpSecret] = useState(urlSec);
  const [showSecret, setShowSecret] = useState(false);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusData, setStatusData] = useState<SafeguardingStatusResponse | null>(null);

  // Messaging state
  const [newMessage, setNewMessage] = useState("");
  const [sendingMessage, setSendingMessage] = useState(false);
  const [messageSuccess, setMessageSuccess] = useState<string | null>(null);

  // Auto-fetch if both ref and sec are in URL
  useEffect(() => {
    if (urlRef && urlSec) {
      fetchStatus(urlRef, urlSec);
    }
  }, [urlRef, urlSec]);

  const fetchStatus = async (ref: string, sec: string) => {
    setError(null);
    setLoading(true);
    try {
      const res = await apiRequest<SafeguardingStatusResponse>("/safeguarding/reports/status", {
        method: "POST",
        body: JSON.stringify({
          reference_number: ref.trim(),
          follow_up_secret: sec.trim(),
        }),
      });
      setStatusData(res);
    } catch (err: any) {
      setError(err.message || "Invalid reference number or secret passkey. Please check your credentials.");
      setStatusData(null);
    } finally {
      setLoading(false);
    }
  };

  const handleLoginSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!referenceNumber.trim() || !followUpSecret.trim()) {
      setError("Please provide both your Case Reference Number and Follow-Up Secret Passkey.");
      return;
    }
    fetchStatus(referenceNumber, followUpSecret);
  };

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMessage.trim() || !statusData) return;

    setSendingMessage(true);
    setMessageSuccess(null);
    setError(null);

    const typedMsg = newMessage.trim();
    const sentMsg: SafeguardingMessageItem = {
      id: "msg-" + Date.now(),
      sender_role: "reporter",
      author_type: "reporter",
      content: typedMsg,
      message: typedMsg,
      created_at: new Date().toISOString(),
    };

    try {
      const res = await apiRequest<{ status: string; message: SafeguardingMessageItem }>(
        "/safeguarding/reports/messages",
        {
          method: "POST",
          body: JSON.stringify({
            reference_number: referenceNumber.trim(),
            follow_up_secret: followUpSecret.trim(),
            message: typedMsg,
          }),
        }
      );

      const msgToAppend = (res && res.message && typeof res.message === "object") ? res.message : sentMsg;

      // Append new message to local view
      setStatusData((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          messages: [...(prev.messages || []), msgToAppend],
        };
      });
      setNewMessage("");
      setMessageSuccess("Your message has been securely sent to the safeguarding officers.");
      setTimeout(() => setMessageSuccess(null), 4000);
      // Synchronize in background
      fetchStatus(referenceNumber, followUpSecret);
    } catch (err: any) {
      setError(err.message || "Failed to send message. Please try again.");
    } finally {
      setSendingMessage(false);
    }
  };

  const handleClear = () => {
    setStatusData(null);
    setReferenceNumber("");
    setFollowUpSecret("");
    setError(null);
  };

  const quickExit = () => {
    window.location.replace("https://www.google.com");
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "submitted":
        return <Badge variant="neutral">Submitted</Badge>;
      case "triaged":
        return <Badge variant="warning">Triaged / Review Pending</Badge>;
      case "under_investigation":
        return <Badge variant="primary">Under Active Investigation</Badge>;
      case "action_taken":
        return <Badge variant="primary">Action Taken</Badge>;
      case "resolved":
        return <Badge variant="success">Resolved</Badge>;
      case "closed":
        return <Badge variant="neutral">Closed</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Quick Exit and Safety Bar */}
      <div className="flex items-center justify-between bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-950">
        <div className="flex items-center gap-2">
          <LifeBuoy className="w-4 h-4 text-amber-700 shrink-0" />
          <span>Need to leave quickly for your safety?</span>
        </div>
        <button
          type="button"
          onClick={quickExit}
          className="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded-lg font-bold text-xs transition"
          title="Instantly exit this portal and load Google"
        >
          Quick Exit
        </button>
      </div>

      {!statusData ? (
        /* Credential Entry Form */
        <div className="bg-white rounded-3xl border border-gray-200 shadow-sm p-6 sm:p-10 space-y-6">
          <div className="text-center space-y-2">
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-800 mx-auto flex items-center justify-center">
              <Lock className="w-6 h-6" />
            </div>
            <h1 className="text-2xl font-black text-gray-900">
              Check Confidential Report Status
            </h1>
            <p className="text-xs text-gray-500 max-w-md mx-auto">
              Enter the Case Reference Number and Follow-Up Secret Passkey issued to you when you submitted your incident report.
            </p>
          </div>

          {error && <Alert type="error">{error}</Alert>}

          <form onSubmit={handleLoginSubmit} className="space-y-4 max-w-lg mx-auto">
            <div>
              <label className="block text-xs font-bold text-gray-700 uppercase tracking-wider mb-1">
                Case Reference Number
              </label>
              <input
                type="text"
                required
                placeholder="e.g. CASE-2026-B817E294"
                value={referenceNumber}
                onChange={(e) => setReferenceNumber(e.target.value)}
                className="w-full px-4 py-2.5 text-sm font-mono border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-600 focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-xs font-bold text-gray-700 uppercase tracking-wider mb-1">
                Follow-Up Secret Passkey
              </label>
              <div className="relative">
                <input
                  type={showSecret ? "text" : "password"}
                  required
                  placeholder="e.g. sec_..."
                  value={followUpSecret}
                  onChange={(e) => setFollowUpSecret(e.target.value)}
                  className="w-full px-4 py-2.5 pr-10 text-sm font-mono border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                />
                <button
                  type="button"
                  onClick={() => setShowSecret(!showSecret)}
                  className="absolute right-3 top-3 text-gray-400 hover:text-gray-600"
                >
                  {showSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <div className="pt-2">
              <button
                type="submit"
                disabled={loading}
                className="w-full py-3 rounded-xl bg-emerald-800 hover:bg-emerald-700 disabled:bg-emerald-400 text-white font-bold text-xs uppercase tracking-wider transition shadow-sm"
              >
                {loading ? "Verifying Credentials..." : "Access Report Status"}
              </button>
            </div>

            <div className="text-center pt-2">
              <Link
                href="/report-incident"
                className="text-xs text-emerald-800 hover:underline font-medium"
              >
                Need to submit a new report instead?
              </Link>
            </div>
          </form>
        </div>
      ) : (
        /* Status Dashboard & Messaging View */
        <div className="space-y-6">
          {/* Header Card */}
          <div className="bg-white rounded-3xl border border-gray-200 shadow-sm p-6 sm:p-8 space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-6 border-b border-gray-100">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs uppercase font-bold text-gray-400 tracking-wider">
                    Confidential Incident Record
                  </span>
                  {getStatusBadge(statusData.status)}
                </div>
                <h1 className="text-xl sm:text-2xl font-black text-gray-900 font-mono">
                  {statusData.public_reference || statusData.reference_number}
                </h1>
                <p className="text-xs text-gray-500">
                  Submitted on {new Date(statusData.submitted_at || statusData.created_at || Date.now()).toLocaleDateString("en-NG", {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </p>
              </div>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleClear}
                  className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl border border-gray-300 hover:bg-gray-50 text-gray-700 text-xs font-semibold transition"
                >
                  <LogOut className="w-4 h-4" />
                  Exit View
                </button>
              </div>
            </div>

            {/* Case Attributes Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs">
              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="text-gray-500 block">Category</span>
                <span className="font-bold text-gray-800 capitalize block">
                  {statusData.category ? statusData.category.replace(/_/g, " ") : "Safeguarding"}
                </span>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="text-gray-500 block">Triage Priority</span>
                <span className="font-bold text-gray-800 capitalize block">
                  {statusData.risk_level || statusData.urgency || "Normal"}
                </span>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="text-gray-500 block">Minor Safeguarding</span>
                <span className="font-bold text-gray-800 block">
                  {statusData.is_minor_involved ? "Yes (Protected Protocol)" : "No"}
                </span>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="text-gray-500 block">Last Status Update</span>
                <span className="font-bold text-gray-800 block">
                  {new Date(statusData.last_status_update_at || statusData.submitted_at || statusData.created_at || Date.now()).toLocaleDateString()}
                </span>
              </div>
            </div>

            {/* Resolution Summary (if resolved or closed) */}
            {statusData.resolution_summary && (
              <div className="p-5 bg-emerald-50 border border-emerald-200 rounded-2xl space-y-2">
                <div className="flex items-center gap-2 text-xs font-bold text-emerald-950 uppercase tracking-wider">
                  <CheckCircle2 className="w-4 h-4 text-emerald-700" />
                  Official Case Outcome & Resolution Summary
                </div>
                <p className="text-xs sm:text-sm text-emerald-900 leading-relaxed">
                  {statusData.resolution_summary}
                </p>
              </div>
            )}
          </div>

          {/* Two-Way Encrypted Messaging */}
          <div className="bg-white rounded-3xl border border-gray-200 shadow-sm p-6 sm:p-8 space-y-6">
            <div className="flex items-center justify-between pb-4 border-b border-gray-100">
              <div className="space-y-1">
                <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <MessageSquare className="w-5 h-5 text-emerald-700" />
                  Confidential Communication Channel
                </h2>
                <p className="text-xs text-gray-500">
                  Send follow-up details, evidence descriptions, or reply to inquiries from the review team.
                </p>
              </div>
            </div>

            {messageSuccess && <Alert type="success">{messageSuccess}</Alert>}
            {error && <Alert type="error">{error}</Alert>}

            {/* Message Thread */}
            <div className="space-y-4 max-h-96 overflow-y-auto p-2">
              {!statusData.messages || statusData.messages.length === 0 ? (
                <div className="text-center py-8 text-xs text-gray-400">
                  No messages exchanged yet. Use the box below to send additional information to the investigation team.
                </div>
              ) : (
                statusData.messages.map((msg) => {
                  const isReporter = msg.sender_role === "reporter" || msg.author_type === "reporter";
                  return (
                    <div
                      key={msg.id}
                      className={`flex flex-col ${isReporter ? "items-end" : "items-start"}`}
                    >
                      <div
                        className={`max-w-[85%] rounded-2xl p-4 text-xs space-y-1.5 shadow-sm ${
                          isReporter
                            ? "bg-emerald-800 text-white rounded-br-none"
                            : "bg-gray-100 text-gray-900 rounded-bl-none border border-gray-200"
                        }`}
                      >
                        <div
                          className={`font-bold text-[10px] uppercase tracking-wider ${
                            isReporter ? "text-emerald-200" : "text-emerald-800"
                          }`}
                        >
                          {isReporter ? "You (Reporter)" : "Safeguarding Team"}
                        </div>
                        <p className="leading-relaxed whitespace-pre-wrap">{msg.content || msg.message}</p>
                        <div
                          className={`text-[9px] text-right ${
                            isReporter ? "text-emerald-300" : "text-gray-400"
                          }`}
                        >
                          {new Date(msg.created_at).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                            month: "short",
                            day: "numeric",
                          })}
                        </div>
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            {/* Send Message Input */}
            <form onSubmit={handleSendMessage} className="space-y-3 pt-4 border-t border-gray-100">
              <label className="block text-xs font-bold text-gray-700">
                Send a Message to the Safeguarding Team
              </label>
              <textarea
                rows={3}
                required
                value={newMessage}
                onChange={(e) => setNewMessage(e.target.value)}
                placeholder="Type additional details, clarify dates, or answer questions..."
                className="w-full p-3 text-xs border border-gray-300 rounded-xl focus:ring-2 focus:ring-emerald-600 focus:outline-none"
              />
              <div className="flex justify-end">
                <button
                  type="submit"
                  disabled={sendingMessage || !newMessage.trim()}
                  className="inline-flex items-center gap-1.5 px-5 py-2.5 rounded-xl bg-emerald-800 hover:bg-emerald-700 disabled:bg-gray-300 text-white font-bold text-xs uppercase tracking-wider transition"
                >
                  <Send className="w-3.5 h-3.5" />
                  {sendingMessage ? "Sending..." : "Send Message"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

export default function IncidentStatusPage() {
  return (
    <Suspense fallback={<Spinner text="Loading status portal..." />}>
      <StatusContent />
    </Suspense>
  );
}
