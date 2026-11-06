export function cleanupIsDone(event: {status: string; end_time: string}, now = Date.now()): boolean {
  return event.status === "completed" || (event.status === "published" && Date.parse(event.end_time) <= now);
}
