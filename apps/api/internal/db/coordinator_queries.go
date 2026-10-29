package db

import (
	"context"
	"encoding/json"
	"strings"
)

func (q *Queries) LockMaterialRequest(ctx context.Context, id string) error {
	var found string
	return q.db.QueryRow(ctx, `SELECT id FROM material_requests WHERE id=$1 FOR UPDATE`, id).Scan(&found)
}
func (q *Queries) LockMaterialTransfer(ctx context.Context, id string) error {
	var found string
	return q.db.QueryRow(ctx, `SELECT id FROM material_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&found)
}
func (q *Queries) ListScopedStockLedgerEntries(ctx context.Context, arg ListStockLedgerEntriesParams, ids []string) ([]ListStockLedgerEntriesRow, error) {
	sql := strings.Replace(listStockLedgerEntries, "ORDER BY", "AND le.location_id=ANY($8::text[]) ORDER BY", 1)
	rows, err := q.db.Query(ctx, "SELECT to_jsonb(entry) FROM ("+sql+") entry", arg.Column1, arg.Column2, arg.Column3, arg.Column4, arg.Column5, arg.Limit, arg.Offset, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListStockLedgerEntriesRow{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row ListStockLedgerEntriesRow
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (q *Queries) CountScopedStockLedgerEntries(ctx context.Context, arg CountStockLedgerEntriesParams, ids []string) (int64, error) {
	var count int64
	err := q.db.QueryRow(ctx, countStockLedgerEntries+` AND le.location_id=ANY($6::text[])`, arg.Column1, arg.Column2, arg.Column3, arg.Column4, arg.Column5, ids).Scan(&count)
	return count, err
}

func (q *Queries) ListScopedReportSnapshots(ctx context.Context, arg ListReportSnapshotsAdminParams, states, lgas, zones []string) ([]ListReportSnapshotsAdminRow, error) {
	condition := ` AND ((rs.scope_type='lga' AND rs.lga_id=ANY($7::text[])) OR (rs.scope_type='state' AND rs.state_id=ANY($8::text[])) OR (rs.scope_type='zone' AND rs.zone_id=ANY($9::text[]))) `
	sql := strings.Replace(listReportSnapshotsAdmin, "ORDER BY", condition+"ORDER BY", 1)
	rows, err := q.db.Query(ctx, "SELECT to_jsonb(entry) FROM ("+sql+") entry", arg.Status, arg.ReportType, arg.ScopeType, arg.StateID, arg.OffsetCount, arg.LimitCount, lgas, states, zones)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListReportSnapshotsAdminRow{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row ListReportSnapshotsAdminRow
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (q *Queries) CountScopedReportSnapshots(ctx context.Context, arg CountReportSnapshotsAdminParams, states, lgas, zones []string) (int64, error) {
	var count int64
	err := q.db.QueryRow(ctx, countReportSnapshotsAdmin+` AND ((scope_type='lga' AND lga_id=ANY($5::text[])) OR (scope_type='state' AND state_id=ANY($6::text[])) OR (scope_type='zone' AND zone_id=ANY($7::text[])))`, arg.Status, arg.ReportType, arg.ScopeType, arg.StateID, lgas, states, zones).Scan(&count)
	return count, err
}

func (q *Queries) LockTransferDiscrepancy(ctx context.Context, id string) error {
	var found string
	return q.db.QueryRow(ctx, `SELECT id FROM transfer_discrepancies WHERE id=$1 FOR UPDATE`, id).Scan(&found)
}
