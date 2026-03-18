package history

import (
	"testing"
	"time"
)

// today returns the current UTC date in YYYY-MM-DD format, matching Store.Append behaviour.
func today() string {
	return time.Now().UTC().Format("2006-01-02")
}

func TestStoreAppend_AddsSnapshot(t *testing.T) {
	s := &Store{}
	taps := map[string]TapSnapshot{
		"ublue-os/homebrew-tap": {Count: 10, Uniques: 5},
	}
	s.Append(taps)

	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(s.Snapshots))
	}
	if s.Snapshots[0].Date != today() {
		t.Errorf("Date = %q, want %q", s.Snapshots[0].Date, today())
	}
	tap := s.Snapshots[0].Taps["ublue-os/homebrew-tap"]
	if tap.Count != 10 || tap.Uniques != 5 {
		t.Errorf("Tap snapshot = %+v, want Count=10 Uniques=5", tap)
	}
}

func TestStoreAppend_Idempotent(t *testing.T) {
	s := &Store{}
	taps := map[string]TapSnapshot{
		"ublue-os/homebrew-tap": {Count: 10, Uniques: 5},
	}
	s.Append(taps)
	// Second Append on the same day must be a no-op.
	s.Append(taps)

	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot after double Append on same day, got %d", len(s.Snapshots))
	}
}

func TestStoreAppend_ChronologicalOrder(t *testing.T) {
	// Seed with two past snapshots that are out of order.
	s := &Store{
		Snapshots: []DaySnapshot{
			{Date: "2026-01-15", Taps: map[string]TapSnapshot{}},
			{Date: "2024-12-31", Taps: map[string]TapSnapshot{}}, // older — out of order
		},
	}
	// Append today's snapshot; Append sorts the full slice after adding.
	s.Append(map[string]TapSnapshot{"tap": {Count: 1}})

	for i := 1; i < len(s.Snapshots); i++ {
		if s.Snapshots[i].Date < s.Snapshots[i-1].Date {
			t.Errorf("snapshots not in chronological order at index %d: %s before %s",
				i, s.Snapshots[i-1].Date, s.Snapshots[i].Date)
		}
	}
	// Also verify today is last (it's the most recent date).
	last := s.Snapshots[len(s.Snapshots)-1].Date
	if last != today() {
		t.Errorf("expected today (%s) to be last snapshot, got %s", today(), last)
	}
}

func TestStoreAppend_EmptyTaps(t *testing.T) {
	s := &Store{}
	s.Append(map[string]TapSnapshot{})
	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot even for empty taps, got %d", len(s.Snapshots))
	}
	if s.Snapshots[0].Taps == nil {
		t.Error("Taps map must not be nil")
	}
}
