"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Bell,
  Mail,
  MessageSquare,
  ShieldCheck,
  Calendar,
  Award,
  ShoppingBag,
  Briefcase,
  Layers,
  DollarSign,
  Megaphone,
  Save,
  ArrowLeft,
  CheckCircle2,
  AlertTriangle,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { NotificationPreferences } from "@/lib/types";

export default function NotificationPreferencesPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [prefs, setPrefs] = useState<NotificationPreferences | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/settings/notifications");
    }
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    const fetchPrefs = async () => {
      setLoading(true);
      try {
        const res = await api.get<NotificationPreferences>("/me/notification-preferences");
        if (res) {
          setPrefs(res);
        }
      } catch (err) {
        console.error("Failed to load preferences", err);
      } finally {
        setLoading(false);
      }
    };
    fetchPrefs();
  }, [user]);

  const handleToggle = (key: keyof NotificationPreferences) => {
    if (!prefs) return;
    setPrefs({
      ...prefs,
      [key]: !prefs[key],
    });
    setSaveSuccess(false);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!prefs) return;

    setSaving(true);
    setSaveError(null);
    setSaveSuccess(false);

    try {
      const res = await api.put<NotificationPreferences>("/me/notification-preferences", {
        in_app_enabled: prefs.in_app_enabled,
        email_enabled: prefs.email_enabled,
        sms_enabled: prefs.sms_enabled,
        cleanup_reminders_enabled: prefs.cleanup_reminders_enabled,
        leadership_alerts_enabled: prefs.leadership_alerts_enabled,
        participation_updates_enabled: prefs.participation_updates_enabled,
        marketplace_updates_enabled: prefs.marketplace_updates_enabled,
        careers_updates_enabled: prefs.careers_updates_enabled,
        community_project_updates_enabled: prefs.community_project_updates_enabled,
        finance_alerts_enabled: prefs.finance_alerts_enabled,
        announcements_enabled: prefs.announcements_enabled,
      });

      if (res) {
        setPrefs(res);
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 4000);
      }
    } catch (err: any) {
      setSaveError(err?.message || "Failed to save preferences");
    } finally {
      setSaving(false);
    }
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
      <div className="max-w-3xl mx-auto space-y-6">
        {/* Navigation & Header */}
        <div className="flex items-center gap-2">
          <Link
            href="/notifications"
            className="p-1.5 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition"
          >
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-xl sm:text-2xl font-bold text-gray-900">Notification Preferences</h1>
            <p className="text-xs sm:text-sm text-gray-500">
              Customize how and when you receive communications from the Ratels Worldwide Movement.
            </p>
          </div>
        </div>

        {saveSuccess && (
          <div className="p-4 bg-emerald-50 border border-emerald-200 rounded-xl flex items-center gap-3 text-emerald-800 text-sm">
            <CheckCircle2 className="w-5 h-5 flex-shrink-0" />
            <span>Your notification settings and consent declarations have been saved successfully.</span>
          </div>
        )}

        {saveError && (
          <div className="p-4 bg-rose-50 border border-rose-200 rounded-xl flex items-center gap-3 text-rose-800 text-sm">
            <AlertTriangle className="w-5 h-5 flex-shrink-0" />
            <span>{saveError}</span>
          </div>
        )}

        {loading ? (
          <div className="py-20 text-center text-gray-400 text-sm">Loading your preferences...</div>
        ) : prefs ? (
          <form onSubmit={handleSave} className="space-y-6">
            {/* 1. Delivery Channels */}
            <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6 space-y-5">
              <div>
                <h2 className="text-base font-bold text-gray-900">Communication Channels</h2>
                <p className="text-xs text-gray-500 mt-0.5">
                  Choose the platforms where you want to receive active alerts and updates.
                </p>
              </div>

              <div className="divide-y divide-gray-100 space-y-4 pt-1">
                {/* In-App */}
                <div className="flex items-start justify-between gap-4 pt-4 first:pt-0">
                  <div className="flex items-start gap-3">
                    <div className="p-2 bg-emerald-50 text-emerald-800 rounded-lg mt-0.5">
                      <Bell className="w-5 h-5" />
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-gray-900">In-App Notifications</p>
                      <p className="text-xs text-gray-500 mt-0.5">
                        Display badges and alerts directly inside the Ratels platform and member dashboard.
                      </p>
                    </div>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={prefs.in_app_enabled}
                      onChange={() => handleToggle("in_app_enabled")}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-emerald-700"></div>
                  </label>
                </div>

                {/* Email */}
                <div className="flex items-start justify-between gap-4 pt-4">
                  <div className="flex items-start gap-3">
                    <div className="p-2 bg-blue-50 text-blue-800 rounded-lg mt-0.5">
                      <Mail className="w-5 h-5" />
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-gray-900">Transactional Email (Brevo)</p>
                      <p className="text-xs text-gray-500 mt-0.5">
                        Receive monthly cleanup notifications, schedules, claims, and appointments to {user.email || "your email"}.
                      </p>
                    </div>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={prefs.email_enabled}
                      onChange={() => handleToggle("email_enabled")}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-emerald-700"></div>
                  </label>
                </div>

                {/* SMS */}
                <div className="pt-4 space-y-3">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex items-start gap-3">
                      <div className="p-2 bg-amber-50 text-amber-800 rounded-lg mt-0.5">
                        <MessageSquare className="w-5 h-5" />
                      </div>
                      <div>
                        <p className="text-sm font-semibold text-gray-900">SMS Alerts & Reminders (Termii)</p>
                        <p className="text-xs text-gray-500 mt-0.5">
                          Receive critical cleanup day reminders and emergency logistics via SMS to {user.phone || "your Nigerian phone number"}.
                        </p>
                      </div>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input
                        type="checkbox"
                        checked={prefs.sms_enabled}
                        onChange={() => handleToggle("sms_enabled")}
                        className="sr-only peer"
                      />
                      <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-emerald-700"></div>
                    </label>
                  </div>

                  {/* NDPR Consent Banner */}
                  <div className="p-3.5 bg-gray-50 border border-gray-200 rounded-xl text-xs text-gray-600 leading-relaxed flex items-start gap-2.5">
                    <ShieldCheck className="w-4 h-4 text-emerald-700 flex-shrink-0 mt-0.5" />
                    <div>
                      <span className="font-semibold text-gray-800">NDPA / NDPR SMS Consent Notice: </span>
                      Enabling SMS gives consent under the Nigeria Data Protection Act for the Ratels Worldwide Movement to dispatch automated event reminders and squad logistics to your number.
                      {prefs.sms_consent_recorded_at && (
                        <p className="mt-1 text-[11px] text-emerald-800 font-medium">
                          Consent recorded on: {new Date(prefs.sms_consent_recorded_at).toLocaleDateString("en-NG", { dateStyle: "long" })}
                        </p>
                      )}
                      {prefs.sms_consent_withdrawn_at && !prefs.sms_enabled && (
                        <p className="mt-1 text-[11px] text-gray-500">
                          Consent withdrawn on: {new Date(prefs.sms_consent_withdrawn_at).toLocaleDateString("en-NG", { dateStyle: "long" })}
                        </p>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* 2. Notification Topics */}
            <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6 space-y-5">
              <div>
                <h2 className="text-base font-bold text-gray-900">Notification Topics</h2>
                <p className="text-xs text-gray-500 mt-0.5">
                  Select which categories of updates you want to receive across your active channels.
                </p>
              </div>

              <div className="divide-y divide-gray-100 space-y-4 pt-1">
                {/* Cleanups */}
                <div className="flex items-center justify-between gap-4 pt-4 first:pt-0">
                  <div className="flex items-center gap-3">
                    <Calendar className="w-4 h-4 text-emerald-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Monthly Cleanup Reminders</p>
                      <p className="text-xs text-gray-500">7-day and 1-day reminders for registered and local events.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.cleanup_reminders_enabled}
                    onChange={() => handleToggle("cleanup_reminders_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Leadership */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <ShieldCheck className="w-4 h-4 text-amber-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Leadership & Jurisdictional Alerts</p>
                      <p className="text-xs text-gray-500">Appointments, role assignments, and governance notices.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.leadership_alerts_enabled}
                    onChange={() => handleToggle("leadership_alerts_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Participation */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <Award className="w-4 h-4 text-blue-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Participation & Verification Claims</p>
                      <p className="text-xs text-gray-500">Approvals, rejections, appeals, and points updates.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.participation_updates_enabled}
                    onChange={() => handleToggle("participation_updates_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Marketplace */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <ShoppingBag className="w-4 h-4 text-emerald-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Ratels Marketplace Orders</p>
                      <p className="text-xs text-gray-500">Order placement, shipment status, and vendor updates.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.marketplace_updates_enabled}
                    onChange={() => handleToggle("marketplace_updates_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Careers */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <Briefcase className="w-4 h-4 text-indigo-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Careers & Empowerment</p>
                      <p className="text-xs text-gray-500">Job application review outcomes and status notifications.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.careers_updates_enabled}
                    onChange={() => handleToggle("careers_updates_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Community Projects */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <Layers className="w-4 h-4 text-purple-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Community Projects & Requests</p>
                      <p className="text-xs text-gray-500">Community proposal reviews, milestones, and progress updates.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.community_project_updates_enabled}
                    onChange={() => handleToggle("community_project_updates_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Finance */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <DollarSign className="w-4 h-4 text-emerald-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">Financial Notices</p>
                      <p className="text-xs text-gray-500">Allocations, disbursements, and accountability receipts.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.finance_alerts_enabled}
                    onChange={() => handleToggle("finance_alerts_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>

                {/* Announcements */}
                <div className="flex items-center justify-between gap-4 pt-4">
                  <div className="flex items-center gap-3">
                    <Megaphone className="w-4 h-4 text-rose-700" />
                    <div>
                      <p className="text-sm font-medium text-gray-900">National & Zonal Announcements</p>
                      <p className="text-xs text-gray-500">Major campaigns, foundation press statements, and alerts.</p>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={prefs.announcements_enabled}
                    onChange={() => handleToggle("announcements_enabled")}
                    className="w-4 h-4 text-emerald-700 rounded border-gray-300 focus:ring-emerald-700"
                  />
                </div>
              </div>
            </div>

            {/* Save Button */}
            <div className="flex items-center justify-end gap-3 pt-2">
              <Link
                href="/notifications"
                className="px-4 py-2.5 rounded-xl border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 transition"
              >
                Cancel
              </Link>
              <button
                type="submit"
                disabled={saving}
                className="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-emerald-800 text-white text-sm font-bold shadow-sm hover:bg-emerald-900 disabled:opacity-50 transition"
              >
                <Save className="w-4 h-4" />
                {saving ? "Saving Changes..." : "Save Preferences"}
              </button>
            </div>
          </form>
        ) : null}
      </div>
    </div>
  );
}
