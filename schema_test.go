package lib

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// migratedTables lists every model InitializeDatabase automigrates. A plugin
// that stores state relies on its table existing after nothing more than
// InitializeDatabase, so a model silently dropped from the migration list is a
// data-loss bug on the next release.
var migratedTables = []struct {
	name  string
	model interface{}
}{
	{"zulip_alarms", &ZulipAlarm{}},
	{"issues", &Issue{}},
	{"systemd_units", &SystemdUnit{}},
	{"versions", &Version{}},
	{"news", &News{}},
	{"cron_intervals", &CronInterval{}},
	{"redis_roles", &RedisRole{}},
	{"valkey_roles", &ValkeyRole{}},
	{"patroni_cluster_members", &PatroniClusterMember{}},
}

func TestInitializeDatabaseCreatesDBFile(t *testing.T) {
	if _, err := os.Stat(GlobalConfig.SqliteLocation); err != nil {
		t.Fatalf("expected the sqlite file at %s: %v", GlobalConfig.SqliteLocation, err)
	}

	if DbDir != filepath.Dir(GlobalConfig.SqliteLocation) {
		t.Errorf("DbDir is %q, expected %q", DbDir, filepath.Dir(GlobalConfig.SqliteLocation))
	}
}

func TestInitializeDatabaseMigratesEveryModel(t *testing.T) {
	for _, tbl := range migratedTables {
		if !DB.Migrator().HasTable(tbl.model) {
			t.Errorf("table %s was not migrated", tbl.name)
		}
	}
}

// DiskUsageTracking and EOLNotification live in lib's schema types but are
// migrated by osHealth itself, not by InitializeDatabase. This test documents
// that split so nobody "fixes" it by assuming the table is already there.
func TestInitializeDatabaseLeavesPluginOwnedTablesAlone(t *testing.T) {
	for _, model := range []interface{}{&DiskUsageTracking{}, &EOLNotification{}} {
		if DB.Migrator().HasTable(model) {
			t.Errorf("%T is migrated by InitializeDatabase, but osHealth also migrates it — "+
				"decide which owns it", model)
		}
	}
}

func TestInitializeDatabaseIsIdempotent(t *testing.T) {
	// Plugins call InitializeDatabase on every run, so a second call against an
	// already-migrated file must be a no-op rather than an error.
	if err := InitializeDatabase(); err != nil {
		t.Fatalf("second InitializeDatabase call failed: %v", err)
	}

	for _, tbl := range migratedTables {
		if !DB.Migrator().HasTable(tbl.model) {
			t.Errorf("table %s disappeared after re-initialising", tbl.name)
		}
	}
}

// TestInitializeDatabaseMigratesFromOlderSchema simulates an upgrade: a table
// that predates a column must gain the column rather than fail the migration.
func TestInitializeDatabaseMigratesFromOlderSchema(t *testing.T) {
	// PatroniClusterMember.Timeline was added after the model shipped, so drop
	// the column and let the migrator put it back.
	if !DB.Migrator().HasColumn(&PatroniClusterMember{}, "Timeline") {
		t.Skip("PatroniClusterMember has no Timeline column to exercise")
	}

	if err := DB.Migrator().DropColumn(&PatroniClusterMember{}, "Timeline"); err != nil {
		t.Fatalf("failed to drop the column to simulate an older schema: %v", err)
	}

	if err := InitializeDatabase(); err != nil {
		t.Fatalf("migration from the older schema failed: %v", err)
	}

	if !DB.Migrator().HasColumn(&PatroniClusterMember{}, "Timeline") {
		t.Error("Timeline column was not restored by the migration")
	}
}

func TestSchemaRoundTripsIssueAPIObjects(t *testing.T) {
	// Issue embeds RedmineAPIObject and RedmineDate, which are persisted through
	// custom Value/Scan implementations. If those break, every Redmine issue read
	// back from the DB loses its project and status.
	isClosed := false
	issue := Issue{
		Subject: "schema round trip",
		Service: "libtest",
		Module:  "schema",
		Status:  "down",
		Project: RedmineAPIObject{Id: 7, Name: "monokit"},
		IssueStatus: RedmineAPIObject{
			Id:       5,
			Name:     "Closed",
			IsClosed: &isClosed,
		},
		StartDate:         RedmineDate{Time: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)},
		ProjectIdentifier: GlobalConfig.ProjectIdentifier,
		Hostname:          GlobalConfig.Hostname,
	}

	if err := DB.Create(&issue).Error; err != nil {
		t.Fatalf("failed to insert the issue: %v", err)
	}
	t.Cleanup(func() { DB.Unscoped().Delete(&Issue{}, "table_id = ?", issue.TableId) })

	var read Issue
	if err := DB.First(&read, "table_id = ?", issue.TableId).Error; err != nil {
		t.Fatalf("failed to read the issue back: %v", err)
	}

	if read.Project.Id != 7 || read.Project.Name != "monokit" {
		t.Errorf("Project did not round trip: %+v", read.Project)
	}
	if read.IssueStatus.IsClosed == nil {
		t.Error("IssueStatus.IsClosed round tripped as nil, expected a false pointer")
	} else if *read.IssueStatus.IsClosed {
		t.Error("IssueStatus.IsClosed round tripped as true, expected false")
	}
	if !read.StartDate.Time.Equal(issue.StartDate.Time) {
		t.Errorf("StartDate did not round trip: got %v, want %v", read.StartDate.Time, issue.StartDate.Time)
	}
}
