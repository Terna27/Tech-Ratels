"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Calendar,
  MapPin,
  Clock,
  Users,
  Search,
  Filter,
  ArrowRight,
  AlertCircle,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { EventInstance, State, LGA } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { EmptyState } from "@/components/ui/EmptyState";

export default function ActivitiesPage() {
  const [events, setEvents] = useState<EventInstance[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [selectedState, setSelectedState] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadData() {
      try {
        const [statesData, eventsData] = await Promise.all([
          apiRequest<State[]>("/geography/states"),
          apiRequest<EventInstance[]>("/activities/events?status=published"),
        ]);
        setStates(statesData || []);
        setEvents(eventsData || []);
      } catch (err) {
        console.error("Failed to load activities", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, []);

  const filteredEvents = events.filter((ev) => {
    const matchesState = !selectedState || ev.state_id === selectedState;
    const matchesSearch =
      !search ||
      ev.title.toLowerCase().includes(search.toLowerCase()) ||
      ev.address.toLowerCase().includes(search.toLowerCase()) ||
      ev.landmark.toLowerCase().includes(search.toLowerCase()) ||
      (ev.state_name && ev.state_name.toLowerCase().includes(search.toLowerCase())) ||
      (ev.lga_name && ev.lga_name.toLowerCase().includes(search.toLowerCase()));

    return matchesState && matchesSearch;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      <div className="space-y-3">
        <Badge variant="primary" size="md">Environmental Sanitation</Badge>
        <h1 className="text-3xl font-extrabold text-gray-900 tracking-tight">
          Cleanups & Community Activities
        </h1>
        <p className="text-sm text-gray-600 max-w-2xl leading-relaxed">
          Explore approved venues for our nationwide sanitation exercises held on the last Saturday of each month.
          Only verified, published venues are displayed below.
        </p>
      </div>

      {/* Filter and Search Bar */}
      <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex flex-col md:flex-row gap-4 justify-between items-stretch">
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search venue by landmark, street, or LGA..."
            className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
          />
        </div>

        <div className="flex items-center gap-3">
          <Filter className="w-4 h-4 text-gray-500" />
          <select
            value={selectedState}
            onChange={(e) => setSelectedState(e.target.value)}
            className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
          >
            <option value="">All States & FCT</option>
            {states.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} {s.is_fct ? "(Territory)" : ""}
              </option>
            ))}
          </select>
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading published activities..." />
      ) : filteredEvents.length === 0 ? (
        <EmptyState
          title="No Published Activities Found"
          description={
            selectedState
              ? "No cleanup venues have been published yet for the selected State/FCT. Coordinators must inspect and confirm venues before they appear here."
              : "No activities currently match your filter criteria."
          }
          actionText="Reset Filters"
          onAction={() => {
            setSelectedState("");
            setSearch("");
          }}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredEvents.map((event) => {
            const isFull = event.registered_count >= event.capacity;
            const percentFilled = Math.min(100, Math.round((event.registered_count / event.capacity) * 100));

            return (
              <div
                key={event.id}
                className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm flex flex-col justify-between hover:border-emerald-400 transition"
              >
                <div className="space-y-4">
                  <div className="flex justify-between items-start gap-2">
                    <Badge variant={isFull ? "danger" : "success"} size="sm">
                      {isFull ? "Capacity Reached" : `${event.capacity - event.registered_count} Slots Left`}
                    </Badge>
                    <span className="text-xs text-gray-500 capitalize">{event.programme_type}</span>
                  </div>

                  <h3 className="font-bold text-gray-900 text-lg leading-snug">
                    {event.title}
                  </h3>

                  <div className="space-y-2 text-xs text-gray-600">
                    <div className="flex items-center gap-2">
                      <Clock className="w-4 h-4 text-emerald-700 flex-shrink-0" />
                      <span>
                        {new Date(event.start_time).toLocaleDateString("en-NG", {
                          weekday: "short",
                          year: "numeric",
                          month: "short",
                          day: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                    </div>

                    <div className="flex items-center gap-2">
                      <MapPin className="w-4 h-4 text-emerald-700 flex-shrink-0" />
                      <span>{event.state_name} • {event.lga_name}</span>
                    </div>

                    <div className="text-gray-500 line-clamp-1">
                      <strong>Landmark:</strong> {event.landmark}
                    </div>

                    {event.rescheduled_from && (
                      <div className="p-2 bg-amber-50 rounded border border-amber-200 text-amber-900 text-[11px]">
                        <strong>Rescheduled:</strong> Previously planned for {new Date(event.rescheduled_from).toLocaleDateString()}
                      </div>
                    )}
                  </div>

                  {/* Capacity Bar */}
                  <div className="space-y-1 pt-2">
                    <div className="flex justify-between text-[11px] text-gray-500">
                      <span>Registered Volunteers</span>
                      <span>{event.registered_count} / {event.capacity}</span>
                    </div>
                    <div className="w-full bg-gray-100 rounded-full h-1.5 overflow-hidden">
                      <div
                        className={`h-full rounded-full ${isFull ? "bg-rose-500" : "bg-emerald-600"}`}
                        style={{ width: `${percentFilled}%` }}
                      />
                    </div>
                  </div>
                </div>

                <div className="pt-6 border-t border-gray-100 mt-4">
                  <Link
                    href={`/activities/${event.id}`}
                    className="w-full inline-flex items-center justify-center px-4 py-2.5 text-sm font-semibold text-emerald-950 bg-emerald-100/70 hover:bg-emerald-200/80 rounded-xl transition"
                  >
                    View Details & Venue
                    <ArrowRight className="w-4 h-4 ml-1.5" />
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
