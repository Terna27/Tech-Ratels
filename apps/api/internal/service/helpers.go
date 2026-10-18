package service

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func Text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func TextFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func StringFromText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func StringPtrFromText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func StringVal(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func Timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func TimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func TimeFromTimestamptz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func Float8(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{Valid: false}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

func Float8Val(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

func Int8(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func Int8FromInt64(i int64) pgtype.Int8 {
	return pgtype.Int8{Int64: i, Valid: true}
}

func Int8Val(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	return &i.Int64
}
