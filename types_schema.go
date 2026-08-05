package lib

import (
	"time"

	"gorm.io/gorm"
)

type Version struct {
	gorm.Model
	Id           uint   `gorm:"primaryKey;autoIncrement"`
	Name         string `gorm:"text,unique" json:"name"`
	Version      string `gorm:"text" json:"version"`       // direct version
	VersionMulti string `gorm:"text" json:"version_multi"` // version in json format for software with multiple components
	Status       string `gorm:"text" json:"status"`        // installed, not-installed
}

type SystemdUnit struct {
	gorm.Model
	id                uint   `gorm:"primaryKey"`
	ProjectIdentifier string `gorm:"text"` // internal use
	Hostname          string `gorm:"text"` // internal use
	Name              string `gorm:"text,unique"`
	LoadState         string `gorm:"text"`
	ActiveState       string `gorm:"text"`
	SubState          string `gorm:"text"`
	Uptime            int64  `gorm:"int"`
	Description       string `gorm:"text"`
}

type ZulipAlarm struct {
	gorm.Model
	Id                uint   `gorm:"primaryKey"`
	ProjectIdentifier string `gorm:"text"` // internal use
	Hostname          string `gorm:"text"` // internal use
	Content           string `gorm:"text"`
	Status            string `gorm:"text"` // down or up
	Service           string `gorm:"text"` // plugin name
	Module            string `gorm:"text"` // plugin's module name
}

type Issue struct {
	gorm.Model
	TableId      uint   `gorm:"primaryKey;autoIncrement"`
	Id           int    `gorm:"int" json:"id,omitempty"`
	Notes        string `gorm:"text" json:"notes,omitempty"`
	ProjectId    string `gorm:"text" json:"project_id,omitempty"`
	TrackerId    int    `gorm:"int" json:"tracker_id,omitempty"`
	Description  string `gorm:"text" json:"description,omitempty"`
	Subject      string `gorm:"text" json:"subject,omitempty"`
	PriorityId   int    `gorm:"int" json:"priority_id,omitempty"`
	StatusId     int    `gorm:"int" json:"status_id,omitempty"`
	AssignedToId string `gorm:"text" json:"assigned_to_id,omitempty"`

	Project             RedmineAPIObject  `gorm:"text" json:"project,omitempty"`
	Tracker             RedmineAPIObject  `gorm:"text" json:"tracker,omitempty"`
	IssueStatus         RedmineAPIObject  `gorm:"text" json:"status,omitempty"`
	Priority            RedmineAPIObject  `gorm:"text" json:"priority,omitempty"`
	Author              RedmineAPIObject  `gorm:"text" json:"author,omitempty"`
	AssignedTo          *RedmineAPIObject `gorm:"text" json:"assigned_to,omitempty"`
	StartDate           RedmineDate       `gorm:"time" json:"start_date,omitempty"`
	DueDate             *RedmineDate      `gorm:"time" json:"due_date,omitempty"`
	DoneRatio           int               `gorm:"int" json:"done_ratio,omitempty"`
	IsPrivate           bool              `gorm:"bool" json:"is_private,omitempty"`
	EstimatedHours      *time.Time        `gorm:"time" json:"estimated_hours,omitempty"`
	TotalEstimatedHours *time.Time        `gorm:"time" json:"total_estimated_hours,omitempty"`
	SpentHours          float64           `gorm:"float" json:"spent_hours,omitempty"`
	TotalSpentHours     float64           `gorm:"float" json:"total_spent_hours,omitempty"`
	CreatedOn           time.Time         `gorm:"time" json:"created_on,omitempty"`
	UpdatedOn           time.Time         `gorm:"time" json:"updated_on,omitempty"`
	ClosedOn            *time.Time        `gorm:"time" json:"closed_on,omitempty"`

	ProjectIdentifier string `gorm:"text"`          // internal use
	Hostname          string `gorm:"text"`          // internal use
	Status            string `gorm:"text" json:"-"` // down or up
	Service           string `gorm:"text" json:"-"` // plugin name
	Module            string `gorm:"text" json:"-"` // plugin's module name
}

type News struct {
	gorm.Model
	TableId           uint   `gorm:"primaryKey;autoIncrement"`
	Id                int    `gorm:"int" json:"id,omitempty"`
	Title             string `gorm:"text" json:"title,omitempty"`
	Description       string `gorm:"text" json:"description,omitempty"`
	ProjectIdentifier string `gorm:"text" json:"-"` // internal use
	Hostname          string `gorm:"text" json:"-"` // internal use
}

// CronInterval keeps track of the last run time of various cron jobs
type CronInterval struct {
	gorm.Model
	Id      uint       `gorm:"primaryKey;autoIncrement"`
	Name    string     `gorm:"text,unique" json:"name"`
	LastRun *time.Time `gorm:"int" json:"last_run"`
}

// PatroniClusterMember stores the last seen state of one Patroni cluster
// member, so pgsqlHealth can detect role changes and cluster size changes
// between runs. One row per member; rows are replaced on every run.
type PatroniClusterMember struct {
	gorm.Model
	Id       uint   `gorm:"primaryKey;autoIncrement"`
	Scope    string `gorm:"text" json:"scope"` // Patroni cluster name
	Name     string `gorm:"text;uniqueIndex" json:"name"`
	Role     string `gorm:"text" json:"role"`
	State    string `gorm:"text" json:"state"`
	Host     string `gorm:"text" json:"host"`
	Port     int64  `gorm:"int" json:"port"`
	Timeline int64  `gorm:"int" json:"timeline"`
}
