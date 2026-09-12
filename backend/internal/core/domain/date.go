package domain

import (
	"encoding/json"
	"time"
)

const dateLayout = "2006-01-02"

// BusinessLocation is the timezone "today" is computed in (Today()) — the
// scheduling domain here is Vietnam-specific (sáng/đêm shifts, VN gates), so
// "today" must reflect Asia/Ho_Chi_Minh rather than whatever timezone the
// server process happens to run in (typically UTC in a container), or the
// default roster/availability window would be off by a day for roughly a
// third of every UTC calendar day. Falls back to UTC if tzdata isn't
// available in a minimal container image, rather than failing startup.
var BusinessLocation = loadBusinessLocation()

func loadBusinessLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.UTC
	}
	return loc
}

// Today returns the current calendar date in BusinessLocation.
func Today() Date {
	return NewDate(time.Now().In(BusinessLocation))
}

// Date is a calendar date with no time-of-day component, (un)marshaled as
// "YYYY-MM-DD" to match solver-service's Pydantic `date` fields exactly.
// It implements TextMarshaler/TextUnmarshaler so it can be used both as a
// plain JSON value and as a JSON object key (e.g. AvailabilityMap).
type Date struct {
	time.Time
}

func NewDate(t time.Time) Date {
	y, m, d := t.Date()
	return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

func (d Date) AddDays(n int) Date {
	return NewDate(d.Time.AddDate(0, 0, n))
}

func (d Date) String() string {
	return d.Format(dateLayout)
}

func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.Format(dateLayout)), nil
}

// AppendText must be defined explicitly for the same reason MarshalJSON is
// below: time.Time implements encoding.TextAppender (RFC3339), and
// encoding/json's map-key encoder prefers TextAppender over TextMarshaler,
// so without this override a map[Date]V would silently encode keys via the
// promoted time.Time.AppendText instead of our "YYYY-MM-DD" format.
func (d Date) AppendText(b []byte) ([]byte, error) {
	return d.Time.AppendFormat(b, dateLayout), nil
}

func (d *Date) UnmarshalText(b []byte) error {
	t, err := time.Parse(dateLayout, string(b))
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// MarshalJSON/UnmarshalJSON must be defined explicitly: Date embeds
// time.Time, which itself implements json.Marshaler/Unmarshaler (RFC3339).
// encoding/json checks that interface before TextMarshaler/TextUnmarshaler,
// so without these overrides the promoted time.Time methods would win for
// plain field values (map keys are unaffected — encoding/json only ever
// consults TextMarshaler for those, so MarshalText above already governs).
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(dateLayout))
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}
