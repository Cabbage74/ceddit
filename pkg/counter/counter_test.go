package counter

import (
	"testing"
	"time"
)

func TestNewPostLikeEvent(t *testing.T) {
	evt := NewPostLikeEvent(456, 123, 1, 0)

	if evt.EntityType != "post" {
		t.Errorf("EntityType = %q, want %q", evt.EntityType, "post")
	}
	if evt.EntityID != 456 {
		t.Errorf("EntityID = %d, want %d", evt.EntityID, 456)
	}
	if evt.Metric != "like" {
		t.Errorf("Metric = %q, want %q", evt.Metric, "like")
	}
	if evt.UID != 123 {
		t.Errorf("UID = %d, want %d", evt.UID, 123)
	}
	if evt.Delta != 1 {
		t.Errorf("Delta = %d, want %d", evt.Delta, 1)
	}
	if evt.Version != 1 {
		t.Errorf("Version = %d, want %d", evt.Version, 1)
	}
	if evt.IdempotentKey == "" {
		t.Error("IdempotentKey should not be empty")
	}
	if evt.Timestamp == 0 {
		t.Error("Timestamp should not be zero")
	}

	// Negative delta for unlike.
	evt2 := NewPostLikeEvent(456, 123, -1, 0)
	if evt2.Delta != -1 {
		t.Errorf("Delta = %d, want %d", evt2.Delta, -1)
	}

	// Idempotent keys should differ for different users/posts/timestamps.
	evt3 := NewPostLikeEvent(789, 123, 1, 0)
	if evt.IdempotentKey == evt3.IdempotentKey {
		t.Error("idempotent keys for different posts should differ")
	}
}

func TestAggKey(t *testing.T) {
	key := AggKey("post", 456, "2024100114")
	expected := "agg:v1:post:456:2024100114"
	if key != expected {
		t.Errorf("AggKey = %q, want %q", key, expected)
	}
}

func TestActiveAggSet(t *testing.T) {
	s := ActiveAggSet()
	if s != "active:agg:v1" {
		t.Errorf("ActiveAggSet = %q, want %q", s, "active:agg:v1")
	}
}

func TestTimeSlot(t *testing.T) {
	// 2024-10-01 14:35:00 UTC
	tm := time.Date(2024, 10, 1, 14, 35, 0, 0, time.UTC)
	slot := TimeSlot(tm)
	if slot != "2024100114" {
		t.Errorf("TimeSlot = %q, want %q", slot, "2024100114")
	}

	// Midnight boundary.
	tm2 := time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)
	slot2 := TimeSlot(tm2)
	if slot2 != "2024100100" {
		t.Errorf("TimeSlot = %q, want %q", slot2, "2024100100")
	}

	// Last hour of day.
	tm3 := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	slot3 := TimeSlot(tm3)
	if slot3 != "2024123123" {
		t.Errorf("TimeSlot = %q, want %q", slot3, "2024123123")
	}
}

func TestTimeSlotOf(t *testing.T) {
	// TimeSlotOf uses local time. Verify the format is correct (14 characters,
	// all digits) and that the result for a known local time is as expected.
	now := time.Now()
	ts := now.Unix()
	slot := TimeSlotOf(ts)

	if len(slot) != 10 {
		t.Errorf("TimeSlotOf(%d) = %q, want 10 characters (YYYYMMDDHH)", ts, slot)
	}
	// Verify all characters are digits.
	for _, c := range slot {
		if c < '0' || c > '9' {
			t.Errorf("TimeSlotOf(%d) = %q, expected all digits", ts, slot)
		}
	}

	// Round-trip: the slot should decode back to the same hour in local time.
	parsed, err := time.ParseInLocation("2006010215", slot, time.Local)
	if err != nil {
		t.Fatalf("failed to parse slot %q: %v", slot, err)
	}
	if parsed.Year() != now.Year() || parsed.Month() != now.Month() ||
		parsed.Day() != now.Day() || parsed.Hour() != now.Hour() {
		t.Errorf("round-trip mismatch: %v vs %v", parsed, now)
	}
}

func TestParseAggKey(t *testing.T) {
	tests := []struct {
		key         string
		wantType    string
		wantID      int64
		expectError bool
	}{
		{"agg:v1:post:456:2024100114", "post", 456, false},
		{"agg:v1:user:789:2024100114", "user", 789, false},
		{"invalid", "", 0, true},
		{"agg:v1:post", "", 0, true},
		{"", "", 0, true},
	}

	for _, tt := range tests {
		entityType, entityID, err := ParseAggKey(tt.key)
		if tt.expectError {
			if err == nil {
				t.Errorf("ParseAggKey(%q) expected error, got nil", tt.key)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAggKey(%q) unexpected error: %v", tt.key, err)
			continue
		}
		if entityType != tt.wantType {
			t.Errorf("ParseAggKey(%q) entityType = %q, want %q", tt.key, entityType, tt.wantType)
		}
		if entityID != tt.wantID {
			t.Errorf("ParseAggKey(%q) entityID = %d, want %d", tt.key, entityID, tt.wantID)
		}
	}
}

func TestParseAggKeyRoundTrip(t *testing.T) {
	// Verify that AggKey → ParseAggKey round-trips correctly.
	entityType := "post"
	entityID := int64(999)
	ts := "2024100114"

	key := AggKey(entityType, entityID, ts)
	gotType, gotID, err := ParseAggKey(key)
	if err != nil {
		t.Fatalf("ParseAggKey(%q) error: %v", key, err)
	}
	if gotType != entityType {
		t.Errorf("entityType = %q, want %q", gotType, entityType)
	}
	if gotID != entityID {
		t.Errorf("entityID = %d, want %d", gotID, entityID)
	}
}

func TestSchemaID(t *testing.T) {
	// SchemaID is embedded in all aggregation keys. Changing it would
	// orphan existing buckets — this test documents that expectation.
	if SchemaID != "v1" {
		t.Errorf("SchemaID changed to %q — update migration docs and key builders", SchemaID)
	}
}
