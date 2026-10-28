"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { History, Shield, ArrowLeft, RefreshCw, Filter } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { AuditEvent } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";

export default function AdminAuditPage() {
  const router = useRouter();
  const { user, isCoordinator, isSuperAdmin, isLoading } = useAuth();

  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);

  const loadAudit = async () => {
    setLoading(true);
    try {
      const data = await apiRequest<any>("/admin/audit?limit=100");
      setEvents(Array.isArray(data) ? data : (data?.items || []));
    } catch (err) {
      console.error("Failed to load audit history", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || (!isCoordinator && !isSuperAdmin))) {
      router.push("/login?redirect=/admin/audit");
      return;
    }
    if (user) {
      loadAudit();
    }
  }, [user, isCoordinator, isSuperAdmin, isLoading, router]);

  if (isLoading || loading) return <Spinner text="Loading audit logs..." />;

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
            System Audit Trail
          </h1>
          <p className="text-xs text-gray-600 mt-1">
            Append-only, immutable record of administrative actions, venue publications, and participation reviews.
          </p>
        </div>

        <button
          type="button"
          onClick={loadAudit}
          className="inline-flex items-center gap-1.5 px-3.5 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50 transition"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          Refresh Audit Trail
        </button>
      </div>

      <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50 flex justify-between items-center text-xs">
          <span className="font-semibold text-gray-700">Chronological Audit Log</span>
          <span className="text-gray-500">{events.length} records</span>
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 text-xs">
            <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px]">
              <tr>
                <th className="px-4 py-3 text-left">Timestamp (UTC)</th>
                <th className="px-4 py-3 text-left">Action</th>
                <th className="px-4 py-3 text-left">Resource</th>
                <th className="px-4 py-3 text-left">Actor Role</th>
                <th className="px-4 py-3 text-left">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 font-mono text-[11px]">
              {events.map((ev) => (
                <tr key={ev.id} className="hover:bg-gray-50/60 transition">
                  <td className="px-4 py-3 whitespace-nowrap text-gray-500 font-sans">
                    {new Date(ev.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <span className="font-bold text-gray-900 font-sans">{ev.action}</span>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-gray-600">
                    {ev.resource_type}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <Badge variant="neutral" size="sm">
                      {ev.actor_role || "system"}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-gray-700 max-w-md font-sans text-xs">
                    {ev.details || "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
