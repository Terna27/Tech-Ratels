"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Calendar,
  MapPin,
  Clock,
  PlusCircle,
  CheckCircle2,
  AlertTriangle,
  ArrowLeft,
  Search,
  Filter,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { EventInstance, State, LGA } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function AdminActivitiesPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();

  const [events, setEvents] = useState<EventInstance[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);
  const [loading, setLoading] = useState(true);

  // Create Modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [title, setTitle] = useState("");
  const [stateId, setStateId] = useState("");
  const [lgaId, setLgaId] = useState("");
  const [address, setAddress] = useState("");
  const [landmark, setLandmark] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [capacity, setCapacity] = useState(100);
  const [publicContact, setPublicContact] = useState("");
  const [accessibilityNotes, setAccessibilityNotes] = useState("");
  const [safetyNotes, setSafetyNotes] = useState("");
  const [creating, setCreating] = useState(false);

  // Reschedule Modal
  const [rescheduleEventId, setRescheduleEventId] = useState<string | null>(null);
  const [newStartTime, setNewStartTime] = useState("");
  const [newEndTime, setNewEndTime] = useState("");
  const [rescheduleReason, setRescheduleReason] = useState("");
  const [rescheduling, setRescheduling] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadData = async () => {
    try {
      const [eventsData, statesData] = await Promise.all([
        apiRequest<EventInstance[]>("/activities/events?status=all"),
        apiRequest<State[]>("/geography/states"),
      ]);
      setEvents(eventsData || []);
      setStates(statesData || []);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load events" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/activities");
      return;
    }
    if (user && isCoordinator) {
      loadData();
    }
  }, [user, isCoordinator, isLoading, router]);

  useEffect(() => {
    if (!stateId) {
      setLgas([]);
      return;
    }
    async function loadLgas() {
      try {
        const data = await apiRequest<LGA[]>(`/geography/states/${stateId}/lgas`);
        setLgas(data || []);
      } catch (err) {
        console.error("Failed to load LGAs", err);
      }
    }
    loadLgas();
  }, [stateId]);

  const handleCreateVenue = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setFeedback(null);

    try {
      await apiRequest("/activities/events", {
        method: "POST",
        body: JSON.stringify({
          title,
          programme_type: "cleanup",
          state_id: stateId,
          lga_id: lgaId,
          address,
          landmark,
          start_time: new Date(startTime).toISOString(),
          end_time: new Date(endTime).toISOString(),
          capacity: Number(capacity),
          public_contact: publicContact,
          accessibility_notes: accessibilityNotes || undefined,
          safety_notes: safetyNotes || undefined,
        }),
      });

      setFeedback({ type: "success", message: "Cleanup venue draft created successfully! Review and publish when confirmed." });
      setShowCreateModal(false);
      resetForm();
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to create venue draft" });
    } finally {
      setCreating(false);
    }
  };

  const handlePublish = async (eventId: string) => {
    setFeedback(null);
    try {
      await apiRequest(`/activities/events/${eventId}/publish`, { method: "POST" });
      setFeedback({ type: "success", message: "Venue published! Volunteers can now view and register." });
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to publish venue" });
    }
  };

  const handleReschedule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rescheduleEventId) return;

    setRescheduling(true);
    setFeedback(null);

    try {
      await apiRequest(`/activities/events/${rescheduleEventId}/reschedule`, {
        method: "POST",
        body: JSON.stringify({
          new_start_time: new Date(newStartTime).toISOString(),
          new_end_time: new Date(newEndTime).toISOString(),
          reason: rescheduleReason,
        }),
      });

      setFeedback({ type: "success", message: "Venue successfully rescheduled. Update history recorded." });
      setRescheduleEventId(null);
      setNewStartTime("");
      setNewEndTime("");
      setRescheduleReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Rescheduling failed" });
    } finally {
      setRescheduling(false);
    }
  };

  const resetForm = () => {
    setTitle("");
    setStateId("");
    setLgaId("");
    setAddress("");
    setLandmark("");
    setStartTime("");
    setEndTime("");
    setCapacity(100);
    setPublicContact("");
    setAccessibilityNotes("");
    setSafetyNotes("");
  };

  if (isLoading || loading) return <Spinner text="Loading venue administration..." />;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      <Link
        href="/admin"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Admin Workspace
      </Link>

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            Cleanup Venues & Activities Management
          </h1>
          <p className="text-xs text-gray-600 mt-1">
            Create venue drafts, verify facilities, publish locations, and manage schedules.
          </p>
        </div>

        {/* Phase 68.3 participation review link */}
          <Link href="/admin/activities/reviews"
            className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-sm font-semibold border border-emerald-300 bg-emerald-50 text-emerald-950 hover:bg-emerald-100 transition">
            Review Participation Claims
          </Link>
          <button
          type="button"
          onClick={() => setShowCreateModal(true)}
          className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition"
        >
          <PlusCircle className="w-4 h-4" />
          Create Cleanup Venue
        </button>
      </div>

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Create Modal */}
      {showCreateModal && (
        <div className="p-6 bg-white rounded-2xl border-2 border-emerald-300 shadow-xl space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Create New Cleanup Venue Draft</h2>
            <button
              onClick={() => setShowCreateModal(false)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <form onSubmit={handleCreateVenue} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Venue Title *</label>
              <input
                type="text"
                required
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Oshodi Terminal Drainage Clearance"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">State / FCT *</label>
                <select
                  required
                  value={stateId}
                  onChange={(e) => setStateId(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                >
                  <option value="">Select State or Territory</option>
                  {states.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} {s.is_fct ? "(Territory)" : ""}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Local Government / Area Council *</label>
                <select
                  required
                  value={lgaId}
                  onChange={(e) => setLgaId(e.target.value)}
                  disabled={!stateId || lgas.length === 0}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white disabled:bg-gray-100"
                >
                  <option value="">Select Local Area</option>
                  {lgas.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Address *</label>
                <input
                  type="text"
                  required
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  placeholder="Street / Assembly Point address"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Landmark *</label>
                <input
                  type="text"
                  required
                  value={landmark}
                  onChange={(e) => setLandmark(e.target.value)}
                  placeholder="Prominent junction, market, or station"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Start Time (Lagos) *</label>
                <input
                  type="datetime-local"
                  required
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">End Time (Lagos) *</label>
                <input
                  type="datetime-local"
                  required
                  value={endTime}
                  onChange={(e) => setEndTime(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Volunteer Capacity *</label>
                <input
                  type="number"
                  required
                  min={1}
                  value={capacity}
                  onChange={(e) => setCapacity(Number(e.target.value))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Public Coordinator Contact *</label>
              <input
                type="text"
                required
                value={publicContact}
                onChange={(e) => setPublicContact(e.target.value)}
                placeholder="Phone or official coordination email"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Accessibility Notes</label>
                <textarea
                  rows={2}
                  value={accessibilityNotes}
                  onChange={(e) => setAccessibilityNotes(e.target.value)}
                  placeholder="Wheelchair access, parking details..."
                  className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Safety & Gear Notes</label>
                <textarea
                  rows={2}
                  value={safetyNotes}
                  onChange={(e) => setSafetyNotes(e.target.value)}
                  placeholder="Gloves provided, boots required, first aid location..."
                  className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={creating}
                className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-semibold transition"
              >
                {creating ? "Creating Draft..." : "Save Draft Venue"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Reschedule Modal */}
      {rescheduleEventId && (
        <div className="p-6 bg-white rounded-2xl border-2 border-amber-300 shadow-xl space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Reschedule Venue Activity</h2>
            <button
              onClick={() => setRescheduleEventId(null)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <form onSubmit={handleReschedule} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">New Start Time (Lagos) *</label>
                <input
                  type="datetime-local"
                  required
                  value={newStartTime}
                  onChange={(e) => setNewStartTime(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">New End Time (Lagos) *</label>
                <input
                  type="datetime-local"
                  required
                  value={newEndTime}
                  onChange={(e) => setNewEndTime(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Reason for Rescheduling *</label>
              <textarea
                required
                minLength={5}
                rows={2}
                value={rescheduleReason}
                onChange={(e) => setRescheduleReason(e.target.value)}
                placeholder="Explain the reason (e.g. inclement weather, local government coordination, road closure)..."
                className="w-full border border-gray-300 rounded-lg p-2.5 text-xs focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setRescheduleEventId(null)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={rescheduling}
                className="px-5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl text-xs font-semibold transition"
              >
                {rescheduling ? "Updating..." : "Confirm Reschedule"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Events Table */}
      <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50 flex justify-between items-center text-xs">
          <span className="font-semibold text-gray-700">All Scheduled & Draft Venues</span>
          <span className="text-gray-500">{events.length} records</span>
        </div>

        {events.length === 0 ? (
          <div className="p-8 text-center text-xs text-gray-500">
            No activities recorded in your jurisdiction. Click &ldquo;Create Cleanup Venue&rdquo; above.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 text-xs">
              <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px]">
                <tr>
                  <th className="px-4 py-3 text-left">Venue Title</th>
                  <th className="px-4 py-3 text-left">Location</th>
                  <th className="px-4 py-3 text-left">Date & Time</th>
                  <th className="px-4 py-3 text-left">Capacity</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {events.map((ev) => (
                  <tr key={ev.id} className="hover:bg-gray-50/60 transition">
                    <td className="px-4 py-3.5">
                      <div className="font-bold text-gray-900">{ev.title}</div>
                      <div className="text-[11px] text-gray-500 truncate max-w-xs">{ev.address}</div>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <div className="font-medium text-gray-900">{ev.state_name}</div>
                      <div className="text-[11px] text-gray-500">{ev.lga_name}</div>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap text-gray-700">
                      <div>{new Date(ev.start_time).toLocaleDateString()}</div>
                      <div className="text-[11px] text-gray-500">
                        {new Date(ev.start_time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                      </div>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap text-gray-700">
                      {ev.registered_count} / {ev.capacity}
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <Badge
                        variant={ev.status === "published" ? "success" : ev.status === "draft" ? "warning" : "neutral"}
                        size="sm"
                      >
                        {ev.status.toUpperCase()}
                      </Badge>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap text-right space-x-2">
                      {ev.status !== "published" && (
                        <button
                          type="button"
                          onClick={() => handlePublish(ev.id)}
                          className="px-2.5 py-1 bg-emerald-800 hover:bg-emerald-900 text-white font-semibold rounded-md text-[11px] transition"
                        >
                          Publish
                        </button>
                      )}
                      {/* Phase 68.2 attendance link */}
                      <Link href={`/admin/activities/${ev.id}/attendance`}
                        className="inline-block px-2.5 py-1 bg-emerald-50 border border-emerald-200 hover:bg-emerald-100 text-emerald-900 font-semibold rounded-md text-[11px] transition">
                        Verify Attendance
                      </Link>
                      <button
                        type="button"
                        onClick={() => setRescheduleEventId(ev.id)}
                        className="px-2.5 py-1 border border-gray-300 hover:bg-gray-100 text-gray-700 font-semibold rounded-md text-[11px] transition"
                      >
                        Reschedule
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
