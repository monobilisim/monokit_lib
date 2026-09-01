package lib

import (
	"strings"
	"testing"
)

// withZulipLimit sets the alarm limit for one test and restores it afterwards.
// The fixture's limit of 10 would make the throttle tests needlessly long.
func withZulipLimit(t *testing.T, limit int) {
	t.Helper()
	original := GlobalConfig.ZulipAlarm.Limit
	GlobalConfig.ZulipAlarm.Limit = limit
	t.Cleanup(func() { GlobalConfig.ZulipAlarm.Limit = original })
}

// cleanupAlarms removes every alarm row a test created.
func cleanupAlarms(t *testing.T, service string) {
	t.Helper()
	t.Cleanup(func() { DB.Unscoped().Where("service = ?", service).Delete(&ZulipAlarm{}) })
}

func countAlarms(service string) int64 {
	var count int64
	DB.Model(&ZulipAlarm{}).Where("service = ?", service).Count(&count)
	return count
}

func TestSendZulipAlarmRecordsFirstAlarm(t *testing.T) {
	service := "libtest-first"
	cleanupAlarms(t, service)

	if err := SendZulipAlarm("disk usage is 91%", service, "disk", "down"); err != nil {
		t.Fatalf("expected the first alarm to go through, got %v", err)
	}

	var alarm ZulipAlarm
	if err := DB.Where("service = ?", service).First(&alarm).Error; err != nil {
		t.Fatalf("expected a row to be recorded: %v", err)
	}

	if alarm.Content != "disk usage is 91%" {
		t.Errorf("Content = %q", alarm.Content)
	}
	if alarm.Module != "disk" {
		t.Errorf("Module = %q, want %q", alarm.Module, "disk")
	}
	if alarm.Status != "down" {
		t.Errorf("Status = %q, want %q", alarm.Status, "down")
	}
	// The identifier and hostname are what scope the throttle to this host; an
	// unset pair would make every host share one alarm budget.
	if alarm.ProjectIdentifier != GlobalConfig.ProjectIdentifier {
		t.Errorf("ProjectIdentifier = %q, want %q", alarm.ProjectIdentifier, GlobalConfig.ProjectIdentifier)
	}
	if alarm.Hostname != GlobalConfig.Hostname {
		t.Errorf("Hostname = %q, want %q", alarm.Hostname, GlobalConfig.Hostname)
	}
}

func TestSendZulipAlarmStopsAtLimit(t *testing.T) {
	// Without this cap a flapping check would post to Zulip on every run forever.
	service := "libtest-limit"
	cleanupAlarms(t, service)
	withZulipLimit(t, 3)

	for i := 1; i <= 3; i++ {
		if err := SendZulipAlarm("still down", service, "svc", "down"); err != nil {
			t.Fatalf("alarm %d should have been sent, got %v", i, err)
		}
	}

	err := SendZulipAlarm("still down", service, "svc", "down")
	if err == nil {
		t.Fatal("expected the 4th identical alarm to be refused")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("expected a limit error, got %v", err)
	}

	// A refused alarm must not be recorded, or the limit would keep sliding
	// forward and never actually cap anything.
	if got := countAlarms(service); got != 3 {
		t.Errorf("expected 3 recorded alarms, found %d", got)
	}
}

func TestSendZulipAlarmAllowsRecoveryAfterLimit(t *testing.T) {
	// The most important case in this file: once the down-alarm budget is spent,
	// the recovery must still get through. Swallowing it would leave the channel
	// showing a permanently broken service.
	service := "libtest-recovery"
	cleanupAlarms(t, service)
	withZulipLimit(t, 2)

	for i := 1; i <= 2; i++ {
		if err := SendZulipAlarm("down", service, "svc", "down"); err != nil {
			t.Fatalf("down alarm %d should have been sent, got %v", i, err)
		}
	}

	if err := SendZulipAlarm("down", service, "svc", "down"); err == nil {
		t.Fatal("expected the down limit to be reached")
	}

	if err := SendZulipAlarm("back up", service, "svc", "up"); err != nil {
		t.Fatalf("the recovery alarm must not be throttled by the down budget, got %v", err)
	}

	last, err := GetLastZulipAlarm(service, "svc")
	if err != nil {
		t.Fatalf("GetLastZulipAlarm returned an error: %v", err)
	}
	if last.Status != "up" {
		t.Errorf("expected the newest alarm to be the recovery, got status %q", last.Status)
	}
}

func TestGetLastZulipAlarmReturnsNewest(t *testing.T) {
	service := "libtest-newest"
	cleanupAlarms(t, service)
	withZulipLimit(t, 10)

	for _, content := range []string{"first", "second", "third"} {
		if err := SendZulipAlarm(content, service, "svc", "down"); err != nil {
			t.Fatalf("failed to send %q: %v", content, err)
		}
	}

	last, err := GetLastZulipAlarm(service, "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if last.Content != "third" {
		t.Errorf("expected the newest alarm, got %q", last.Content)
	}
}

func TestGetLastZulipAlarmNoRows(t *testing.T) {
	// The throttle calls this before any alarm exists, so "nothing found" has to
	// be an empty struct rather than an error — an error would abort the send.
	last, err := GetLastZulipAlarm("libtest-does-not-exist", "svc")
	if err != nil {
		t.Fatalf("expected no error for a service with no alarms, got %v", err)
	}
	if last.Content != "" || last.Status != "" {
		t.Errorf("expected an empty alarm, got %+v", last)
	}
}

func TestGetLastZulipAlarmsCapsAtConfiguredLimit(t *testing.T) {
	service := "libtest-cap"
	cleanupAlarms(t, service)
	withZulipLimit(t, 10)

	for _, content := range []string{"a", "b", "c", "d"} {
		if err := SendZulipAlarm(content, service, "svc", "down"); err != nil {
			t.Fatalf("failed to send %q: %v", content, err)
		}
	}

	// The window the limit check inspects is exactly Limit rows deep, so it must
	// follow the configured value rather than returning everything.
	GlobalConfig.ZulipAlarm.Limit = 2

	alarms, err := GetLastZulipAlarms(service, "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alarms) != 2 {
		t.Fatalf("expected 2 alarms, got %d", len(alarms))
	}
	if alarms[0].Content != "d" {
		t.Errorf("expected the newest alarm first, got %q", alarms[0].Content)
	}
}

func TestZulipAlarmsAreScopedByServiceAndModule(t *testing.T) {
	// Every plugin shares one alarms table, so a leak here would let one plugin's
	// noise exhaust another's budget.
	cleanupAlarms(t, "libtest-scope-a")
	cleanupAlarms(t, "libtest-scope-b")
	withZulipLimit(t, 10)

	if err := SendZulipAlarm("a/one", "libtest-scope-a", "one", "down"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SendZulipAlarm("a/two", "libtest-scope-a", "two", "down"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SendZulipAlarm("b/one", "libtest-scope-b", "one", "down"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	alarms, err := GetLastZulipAlarms("libtest-scope-a", "one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm for service a / module one, got %d", len(alarms))
	}
	if alarms[0].Content != "a/one" {
		t.Errorf("wrong alarm returned: %q", alarms[0].Content)
	}
}
