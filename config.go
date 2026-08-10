package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

var LogDir string
var DbDir string
var PluginsDir string
var GlobalConfig GlobalConfigType
var OsHealthConfig OsHealthConfigType
var UfwApplyConfig UfwApplyConfigType
var DBConfig DBConfigType
var RedisHealthConfig RedisHealthConfigType

func InitConfig(configFiles ...string) error {
	if _, err := os.Stat("/etc/mono"); os.IsNotExist(err) {
		err := os.MkdirAll("/etc/mono", 0755)
		if err != nil {
			return fmt.Errorf("failed to create /etc/mono directory: %w", err)
		}
	}

	globalConfigExists := false
	if _, err := os.Stat("/etc/mono/global.yml"); err == nil {
		globalConfigExists = true
	} else {
		return fmt.Errorf("global configuration file does not exist")
	}

	if globalConfigExists {
		globalConfigData, err := os.ReadFile("/etc/mono/global.yml")
		if err != nil {
			return fmt.Errorf("failed to read global configuration file: %w", err)
		}

		err = yaml.Unmarshal(globalConfigData, &GlobalConfig)
		if err != nil {
			return fmt.Errorf("failed to parse global configuration file: %w", err)
		}
	}

	// /var/log/monokit2/monokit.log -> /var/log/monokit2
	LogDir = strings.Join(strings.Split(GlobalConfig.LogLocation, "/")[0:len(strings.Split(GlobalConfig.LogLocation, "/"))-1], "/")

	// /var/lib/monokit2/monokit.db -> /var/lib/monokit2
	DbDir = strings.Join(strings.Split(GlobalConfig.SqliteLocation, "/")[0:len(strings.Split(GlobalConfig.SqliteLocation, "/"))-1], "/")

	PluginsDir = GlobalConfig.PluginsLocation

	for _, configFile := range configFiles {
		switch configFile {
		case "os.yml":
			osHealthConfigExists := false
			if _, err := os.Stat("/etc/mono/os.yml"); err == nil {
				osHealthConfigExists = true
			} else {
				return fmt.Errorf("os configuration file does not exist")
			}

			if osHealthConfigExists {
				osHealthConfigData, err := os.ReadFile("/etc/mono/os.yml")
				if err != nil {
					return fmt.Errorf("failed to read os configuration file: %w", err)
				}

				err = yaml.Unmarshal(osHealthConfigData, &OsHealthConfig)
				if err != nil {
					return fmt.Errorf("failed to parse os configuration file: %w", err)
				}
			}
		case "ufw.yml":
			ufwApplyConfigExists := false
			if _, err := os.Stat("/etc/mono/ufw.yml"); err == nil {
				ufwApplyConfigExists = true
			} else {
				return fmt.Errorf("ufw configuration file does not exist")
			}

			if ufwApplyConfigExists {
				ufwApplyConfigData, err := os.ReadFile("/etc/mono/ufw.yml")
				if err != nil {
					return fmt.Errorf("failed to read ufw configuration file: %w", err)
				}

				err = yaml.Unmarshal(ufwApplyConfigData, &UfwApplyConfig)
				if err != nil {
					return fmt.Errorf("failed to parse ufw configuration file: %w", err)
				}
			}
		case "db.yml":
			dbConfigExists := false
			if _, err := os.Stat("/etc/mono/db.yml"); err == nil {
				dbConfigExists = true
			} else {
				return fmt.Errorf("db configuration file does not exist")
			}

			if dbConfigExists {
				dbConfigData, err := os.ReadFile("/etc/mono/db.yml")
				if err != nil {
					return fmt.Errorf("failed to read db configuration file: %w", err)
				}

				err = yaml.Unmarshal(dbConfigData, &DBConfig)
				if err != nil {
					return fmt.Errorf("failed to parse db configuration file: %w", err)
				}
			}
		case "redis.yml":
			redisHealthConfigExists := false
			if _, err := os.Stat("/etc/mono/redis.yml"); err == nil {
				redisHealthConfigExists = true
			} else {
				return fmt.Errorf("redis configuration file does not exist")
			}

			if redisHealthConfigExists {
				redisHealthConfigData, err := os.ReadFile("/etc/mono/redis.yml")
				if err != nil {
					return fmt.Errorf("failed to read redis configuration file: %w", err)
				}

				err = yaml.Unmarshal(redisHealthConfigData, &RedisHealthConfig)
				if err != nil {
					return fmt.Errorf("failed to parse redis configuration file: %w", err)
				}
			}
		}
	}

	return nil
}

func CheckPluginDependencies() []string {
	var missingDependencies []string

	if _, err := os.Stat(PluginsDir); os.IsNotExist(err) {
		err := os.MkdirAll(PluginsDir, 0755)
		if err != nil {
			return []string{fmt.Sprintf("Failed to create plugins directory: %s", err.Error())}
		}
	}

	files, err := os.ReadDir(PluginsDir)
	if err != nil {
		return []string{fmt.Sprintf("Failed to read plugins directory: %s", err.Error())}
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		pluginName := file.Name()
		pluginPath := fmt.Sprintf("%s/%s", PluginsDir, pluginName)

		// Check if file is executable
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0111 == 0 {
			continue
		}

		// Run plugin with -d flag
		cmd := exec.Command(pluginPath, "-d")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		var deps Dependencies
		if err := json.Unmarshal(output, &deps); err != nil {
			continue
		}

		for _, configFile := range deps.ConfigFiles {
			configPath := fmt.Sprintf("/etc/mono/%s", configFile)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				missingDependencies = append(missingDependencies, fmt.Sprintf("%s config is absent, requester: %s", configFile, pluginName))
			}
		}
	}

	return missingDependencies
}
