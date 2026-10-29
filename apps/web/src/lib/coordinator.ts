export type OperationalScope = { national: boolean; zone_ids: string[]; state_ids: string[]; lga_ids: string[] };
export type Area = { state_id: string; state_name: string; lga_id: string; lga_name: string; members: number; events: number; verified_attendance: number };
export type CoordinatorReport = { scope: OperationalScope; summary: Record<string, number>; geography: Area[]; from: string; until: string };
export function calendarInputToUTC(value: string): string {
 const date = new Date(value);
 if (!Number.isFinite(date.getTime())) throw new Error("Enter a valid start and end date/time");
 return date.toISOString();
}
export function correctionPayload(state: string, lga: string, address: string, reason: string) {
 if (!state || !lga || !reason.trim()) throw new Error("State, LGA and a correction reason are required");
 return { home_state_id: state, home_lga_id: lga, address: address.trim(), reason: reason.trim() };
}
