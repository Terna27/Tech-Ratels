"use client";

import React, { useEffect, useState, use } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Calendar,
  MapPin,
  Clock,
  Users,
  Shield,
  Phone,
  ArrowLeft,
  CheckCircle,
  AlertTriangle,
  FileText,
  XCircle,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { EventInstance, EventRegistration } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function ActivityDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const { id } = use(params);
  const { user } = useAuth();

  const [event, setEvent] = useState<EventInstance | null>(null);
  const [isRegistered, setIsRegistered] = useState(false);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  useEffect(() => {
    async function loadEvent() {
      try {
        const data = await apiRequest<EventInstance>(`/activities/events/${id}`);
        setEvent(data);

        if (user) {
          const myRegs = await apiRequest<EventRegistration[]>("/activities/my-registrations");
          const registered = myRegs.some((r) => r.event_id === id && r.status === "registered");
          setIsRegistered(registered);
        }
      } catch (err: any) {
        setFeedback({ type: "error", message: err.message || "Failed to load event details" });
      } finally {
        setLoading(false);
      }
    }
    loadEvent();
  }, [id, user]);

  const handleRegister = async () => {
    if (!user) {
      router.push("/login?redirect=" + encodeURIComponent(`/activities/${id}`));
      return;
    }

    setSubmitting(true);
    setFeedback(null);
    try {
      await apiRequest(`/activities/events/${id}/register`, { method: "POST" });
      setIsRegistered(true);
      setEvent((prev) => prev ? { ...prev, registered_count: prev.registered_count + 1 } : null);
      setFeedback({ type: "success", message: "You have successfully registered for this cleanup venue!" });
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to register" });
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancelRegistration = async () => {
    if (!confirm("Are you sure you want to cancel your registration?")) return;

    setSubmitting(true);
    setFeedback(null);
    try {
      await apiRequest(`/activities/events/${id}/cancel-registration`, { method: "POST" });
      setIsRegistered(false);
      setEvent((prev) => prev ? { ...prev, registered_count: Math.max(0, prev.registered_count - 1) } : null);
      setFeedback({ type: "success", message: "Your registration has been cancelled." });
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to cancel registration" });
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <Spinner text="Loading venue details..." />;
  if (!event) return <div className="text-center py-16 text-gray-500">Event not found.</div>;

  const isFull = event.registered_count >= event.capacity;
  const percentFilled = Math.min(100, Math.round((event.registered_count / event.capacity) * 100));

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      <Link
        href="/activities"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Activities
      </Link>

      {feedback && (
        <Alert type={feedback.type} title={feedback.type === "success" ? "Success" : "Notice"}>
          {feedback.message}
        </Alert>
      )}

      {/* Header card */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Badge variant={event.status === "published" ? "success" : "warning"} size="md">
              {event.status.toUpperCase()}
            </Badge>
            <span className="text-xs text-gray-500 font-medium capitalize">
              {event.programme_type} Programme
            </span>
          </div>

          <div className="text-xs font-medium text-gray-500">
            Registered: <strong className="text-gray-900">{event.registered_count}</strong> of {event.capacity}
          </div>
        </div>

        <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 leading-tight">
          {event.title}
        </h1>

        {/* Rescheduled Notice */}
        {event.rescheduled_from && (
          <Alert type="warning" title="Rescheduled Event">
            This activity was previously scheduled for {new Date(event.rescheduled_from).toLocaleDateString()}.
            <br />
            <strong>Reason:</strong> {event.rescheduled_reason}
          </Alert>
        )}

        {/* Primary Meta Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 flex items-start gap-3">
            <Clock className="w-5 h-5 text-emerald-700 mt-0.5" />
            <div>
              <div className="font-semibold text-gray-900 text-sm">Date & Time (Lagos Time)</div>
              <div className="text-gray-600 mt-0.5">
                {new Date(event.start_time).toLocaleDateString("en-NG", {
                  weekday: "long",
                  year: "numeric",
                  month: "long",
                  day: "numeric",
                })}
              </div>
              <div className="text-gray-500 mt-0.5">
                {new Date(event.start_time).toLocaleTimeString("en-NG", { hour: "2-digit", minute: "2-digit" })} –{" "}
                {new Date(event.end_time).toLocaleTimeString("en-NG", { hour: "2-digit", minute: "2-digit" })}
              </div>
            </div>
          </div>

          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 flex items-start gap-3">
            <MapPin className="w-5 h-5 text-emerald-700 mt-0.5" />
            <div>
              <div className="font-semibold text-gray-900 text-sm">Venue Location</div>
              <div className="text-gray-800 font-medium mt-0.5">{event.address}</div>
              <div className="text-gray-600 mt-0.5">
                Landmark: {event.landmark} ({event.state_name} • {event.lga_name})
              </div>
            </div>
          </div>
        </div>

        {/* Capacity Bar */}
        <div className="space-y-1.5">
          <div className="flex justify-between text-xs text-gray-600">
            <span>Volunteer Capacity</span>
            <span>{percentFilled}% full ({event.capacity - event.registered_count} slots left)</span>
          </div>
          <div className="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
            <div
              className={`h-full rounded-full ${isFull ? "bg-rose-500" : "bg-emerald-600"}`}
              style={{ width: `${percentFilled}%` }}
            />
          </div>
        </div>

        {/* Action Buttons */}
        <div className="pt-4 border-t border-gray-100 flex flex-col sm:flex-row items-center gap-3">
          {isRegistered ? (
            <div className="w-full flex flex-col sm:flex-row items-center justify-between gap-3 p-4 bg-emerald-50 rounded-xl border border-emerald-200">
              <div className="flex items-center gap-2 text-emerald-900 text-xs font-semibold">
                <CheckCircle className="w-5 h-5 text-emerald-700" />
                <span>You are registered for this cleanup!</span>
              </div>
              <div className="flex gap-2">
                <Link
                  href="/member/participations"
                  className="px-3.5 py-2 text-xs font-semibold text-white bg-emerald-800 hover:bg-emerald-900 rounded-lg transition"
                >
                  Submit Attendance Evidence
                </Link>
                <button
                  type="button"
                  onClick={handleCancelRegistration}
                  disabled={submitting}
                  className="px-3.5 py-2 text-xs font-semibold text-rose-700 bg-white border border-rose-200 hover:bg-rose-50 rounded-lg transition"
                >
                  Cancel Registration
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleRegister}
              disabled={submitting || isFull || event.status !== "published"}
              className={`w-full sm:w-auto px-8 py-3.5 rounded-xl text-sm font-semibold shadow-sm transition ${
                isFull
                  ? "bg-gray-200 text-gray-500 cursor-not-allowed"
                  : event.status !== "published"
                  ? "bg-gray-200 text-gray-500 cursor-not-allowed"
                  : "bg-emerald-800 hover:bg-emerald-900 text-white"
              }`}
            >
              {isFull ? "Capacity Reached" : "Register to Attend Cleanup"}
            </button>
          )}
        </div>
      </div>

      {/* Safety & Logistics Notes */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
        <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
          <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
            <Shield className="w-4 h-4 text-emerald-700" />
            Safety & Logistics Instructions
          </h3>
          <p className="text-xs text-gray-600 leading-relaxed">
            {event.safety_notes || "Wear sturdy closed-toe shoes or boots. Bring a refillable water bottle. Protective gloves and waste bags will be distributed on arrival."}
          </p>
        </div>

        <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
          <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
            <Phone className="w-4 h-4 text-emerald-700" />
            Coordinator Contact & Accessibility
          </h3>
          <p className="text-xs text-gray-600 leading-relaxed">
            <strong>Public Contact:</strong> {event.public_contact}
          </p>
          <p className="text-xs text-gray-600 leading-relaxed">
            <strong>Accessibility:</strong> {event.accessibility_notes || "Standard pedestrian access. Contact local coordinator for mobility assistance."}
          </p>
        </div>
      </div>
    </div>
  );
}
