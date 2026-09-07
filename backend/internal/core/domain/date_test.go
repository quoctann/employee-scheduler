package domain

import (
	"encoding/json"
	"testing"
)

// Regression test: Date embeds time.Time, whose promoted MarshalJSON/
// UnmarshalJSON (RFC3339) would otherwise silently win over our own
// "YYYY-MM-DD" TextMarshaler/TextUnmarshaler for plain field values.
func TestDate_JSONRoundTrip_UsesBareDateFormat(t *testing.T) {
	d, err := ParseDate("2026-09-07")
	if err != nil {
		t.Fatalf("ParseDate() error = %v", err)
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != `"2026-09-07"` {
		t.Fatalf("Marshal() = %s, want \"2026-09-07\"", b)
	}

	var got Date
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got != d {
		t.Fatalf("Unmarshal() = %v, want %v", got, d)
	}
}

func TestDate_AsMapKey_RoundTrips(t *testing.T) {
	d, err := ParseDate("2026-09-07")
	if err != nil {
		t.Fatalf("ParseDate() error = %v", err)
	}

	m := map[Date]bool{d: true}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != `{"2026-09-07":true}` {
		t.Fatalf("Marshal() = %s, want {\"2026-09-07\":true}", b)
	}

	var got map[Date]bool
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got[d] {
		t.Fatalf("Unmarshal() = %v, want key %v present", got, d)
	}
}

func TestDate_AddDays(t *testing.T) {
	start, _ := ParseDate("2026-09-07")
	got := start.AddDays(3)
	want, _ := ParseDate("2026-09-10")
	if got != want {
		t.Fatalf("AddDays(3) = %v, want %v", got, want)
	}
}
