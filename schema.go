package lib

import (
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitializeDatabase() error {
	err := os.MkdirAll(DbDir, os.ModePerm)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create database directory")
		return err
	}

	db, err := gorm.Open(sqlite.Open(GlobalConfig.SqliteLocation), &gorm.Config{})
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to connect to database")
		return err
	}

	db.AutoMigrate(&ZulipAlarm{})

	db.AutoMigrate(&Issue{})

	db.AutoMigrate(&SystemdUnit{})

	db.AutoMigrate(&Version{})

	db.AutoMigrate(&News{})

	db.AutoMigrate(&CronInterval{})

	db.AutoMigrate(&RedisRole{})

	db.AutoMigrate(&ValkeyRole{})

	db.AutoMigrate(&PatroniClusterMember{})

	DB = db

	return nil
}
