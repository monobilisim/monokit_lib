package lib

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestIsTestMode(t *testing.T) {
	// IsTestMode gates every outbound network call in the library, so the exact
	// set of accepted values matters: a plugin's CI that sets TEST=on would
	// otherwise post real alarms to Zulip.
	tests := []struct {
		value    string
		set      bool
		expected bool
	}{
		{"1", true, true},
		{"true", true, true},
		{"yes", true, true},
		{"0", true, false},
		{"false", true, false},
		{"on", true, false},
		{"TRUE", true, false}, // case sensitive by design of the current check
		{"", true, false},
		{"", false, false},
	}

	original, hadOriginal := os.LookupEnv("TEST")
	t.Cleanup(func() {
		if hadOriginal {
			os.Setenv("TEST", original)
		} else {
			os.Unsetenv("TEST")
		}
	})

	for _, tt := range tests {
		if tt.set {
			os.Setenv("TEST", tt.value)
		} else {
			os.Unsetenv("TEST")
		}

		if got := IsTestMode(); got != tt.expected {
			t.Errorf("TEST=%q (set=%v): expected %v, got %v", tt.value, tt.set, tt.expected, got)
		}
	}
}

func TestGetLastCronIntervalCreatesRowWhenAbsent(t *testing.T) {
	name := "libtest-cron-absent"
	t.Cleanup(func() { DB.Unscoped().Where("name = ?", name).Delete(&CronInterval{}) })

	ci := GetLastCronInterval(name)

	if ci.Name != name {
		t.Errorf("expected name %q, got %q", name, ci.Name)
	}
	// A fresh row must report "never run", otherwise a throttled check would
	// skip its very first run.
	if ci.LastRun != nil {
		t.Errorf("expected LastRun to be nil for a new interval, got %v", ci.LastRun)
	}

	var count int64
	DB.Model(&CronInterval{}).Where("name = ?", name).Count(&count)
	if count != 1 {
		t.Errorf("expected exactly one row to be created, found %d", count)
	}
}

func TestCreateOrUpdateCronIntervalStampsLastRun(t *testing.T) {
	name := "libtest-cron-stamp"
	t.Cleanup(func() { DB.Unscoped().Where("name = ?", name).Delete(&CronInterval{}) })

	before := time.Now()
	CreateOrUpdateCronInterval(name)

	ci := GetLastCronInterval(name)
	if ci.LastRun == nil {
		t.Fatal("expected LastRun to be set after CreateOrUpdateCronInterval")
	}
	if ci.LastRun.Before(before.Add(-time.Second)) {
		t.Errorf("LastRun %v is older than the call that set it (%v)", ci.LastRun, before)
	}
}

func TestCreateOrUpdateCronIntervalDoesNotDuplicateRows(t *testing.T) {
	// The throttle reads the row back by name, so a second row would make the
	// interval unreliable — First() could return either one.
	name := "libtest-cron-nodupe"
	t.Cleanup(func() { DB.Unscoped().Where("name = ?", name).Delete(&CronInterval{}) })

	CreateOrUpdateCronInterval(name)
	first := GetLastCronInterval(name)

	time.Sleep(10 * time.Millisecond)
	CreateOrUpdateCronInterval(name)
	second := GetLastCronInterval(name)

	var count int64
	DB.Model(&CronInterval{}).Where("name = ?", name).Count(&count)
	if count != 1 {
		t.Fatalf("expected one row after two updates, found %d", count)
	}

	if first.LastRun == nil || second.LastRun == nil {
		t.Fatal("expected both reads to have a LastRun")
	}
	if !second.LastRun.After(*first.LastRun) {
		t.Errorf("second LastRun (%v) should be after the first (%v)", second.LastRun, first.LastRun)
	}
}

func TestRedmineDateUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  time.Time
	}{
		{"redmine date format", `"2026-03-14"`, false, time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)},
		{"rfc3339 fallback", `"2026-03-14T10:20:30Z"`, false, time.Date(2026, 3, 14, 10, 20, 30, 0, time.UTC)},
		{"json null", `null`, false, time.Time{}},
		{"empty string", `""`, false, time.Time{}},
		{"unparseable", `"not a date"`, true, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d RedmineDate
			err := json.Unmarshal([]byte(tt.input), &d)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected an error for %s", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.input, err)
			}
			if !d.Time.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, d.Time)
			}
		})
	}
}

func TestRedmineDateMarshalJSON(t *testing.T) {
	// Redmine rejects a full timestamp on its date fields, so a set date has to
	// serialise as YYYY-MM-DD and an unset one as null rather than 0001-01-01.
	set := RedmineDate{Time: time.Date(2026, 3, 14, 10, 20, 30, 0, time.UTC)}
	out, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != `"2026-03-14"` {
		t.Errorf("expected %q, got %q", `"2026-03-14"`, string(out))
	}

	out, err = json.Marshal(RedmineDate{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "null" {
		t.Errorf("expected null for a zero date, got %q", string(out))
	}
}

func TestRedmineDateValueAndScan(t *testing.T) {
	when := time.Date(2026, 3, 14, 10, 20, 30, 0, time.UTC)

	v, err := RedmineDate{Time: when}.Value()
	if err != nil {
		t.Fatalf("Value returned an error: %v", err)
	}
	if got, ok := v.(time.Time); !ok || !got.Equal(when) {
		t.Errorf("expected Value to return %v, got %v", when, v)
	}

	var d RedmineDate
	if err := d.Scan(when); err != nil {
		t.Fatalf("Scan(time.Time) returned an error: %v", err)
	}
	if !d.Time.Equal(when) {
		t.Errorf("expected %v after Scan, got %v", when, d.Time)
	}

	var nilled RedmineDate
	if err := nilled.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) returned an error: %v", err)
	}
	if !nilled.Time.IsZero() {
		t.Errorf("expected a zero time after Scan(nil), got %v", nilled.Time)
	}

	// A driver handing back a string must be rejected loudly rather than
	// silently leaving the field at its zero value.
	var bad RedmineDate
	if err := bad.Scan("2026-03-14"); err == nil {
		t.Error("expected Scan to reject a string value")
	}
}

func TestRedmineAPIObjectValueAndScan(t *testing.T) {
	isClosed := true
	obj := RedmineAPIObject{Id: 42, Name: "Closed", IsClosed: &isClosed}

	v, err := obj.Value()
	if err != nil {
		t.Fatalf("Value returned an error: %v", err)
	}

	raw, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected Value to return []byte, got %T", v)
	}

	// []byte is what the sqlite driver hands back for a text column.
	var fromBytes RedmineAPIObject
	if err := fromBytes.Scan(raw); err != nil {
		t.Fatalf("Scan([]byte) returned an error: %v", err)
	}
	if fromBytes.Id != 42 || fromBytes.Name != "Closed" {
		t.Errorf("round trip lost data: %+v", fromBytes)
	}
	if fromBytes.IsClosed == nil || !*fromBytes.IsClosed {
		t.Errorf("IsClosed did not round trip: %v", fromBytes.IsClosed)
	}

	// Some drivers hand back a string for the same column.
	var fromString RedmineAPIObject
	if err := fromString.Scan(string(raw)); err != nil {
		t.Fatalf("Scan(string) returned an error: %v", err)
	}
	if fromString.Id != 42 {
		t.Errorf("string scan lost data: %+v", fromString)
	}

	var fromNil RedmineAPIObject
	if err := fromNil.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) returned an error: %v", err)
	}
	if fromNil.Id != 0 || fromNil.Name != "" {
		t.Errorf("expected Scan(nil) to leave the object empty, got %+v", fromNil)
	}

	var fromInt RedmineAPIObject
	if err := fromInt.Scan(42); err == nil {
		t.Error("expected Scan to reject an int value")
	}
}
