// Package converters registers shared type converters for mapgen and
// exposes them for manual use in handlers.
package converters

import (
	"time"

	"github.com/agilsyofian/utils/mapgen/runtime/mapper"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/type/date"
)

func init() {
	mapper.Register(ToDate)
	mapper.Register(UUIDToString)
	mapper.Register(ToTime)
	mapper.RegisterE(uuid.Parse)
}

// UUIDToString renders a UUID in its canonical string form.
func UUIDToString(id uuid.UUID) string {
	return id.String()
}

// ToTime converts a calendar date into a UTC midnight time.
func ToTime(d *date.Date) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}

// ToDate converts a time into its calendar date.
func ToDate(t time.Time) *date.Date {
	year, month, day := t.Date()
	//nolint:gosec // Calendar year, month, and day always fit in int32.
	return &date.Date{Year: int32(year), Month: int32(month), Day: int32(day)}
}
