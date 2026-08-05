package lib

type GlobalConfigType struct {
	ProjectIdentifier string `yaml:"project-identifier"`
	Hostname          string `yaml:"hostname"`
	LogLocation       string `yaml:"log-location"`
	SqliteLocation    string `yaml:"sqlite-location"`
	PluginsLocation   string `yaml:"plugins-location"`

	ZulipAlarm struct {
		Enabled     bool     `yaml:"enabled"`
		Interval    int      `yaml:"interval"`
		WebhookUrls []string `yaml:"webhook-urls"`
		Limit       int      `yaml:"limit"`

		BotApi struct {
			Enabled    bool     `yaml:"enabled"`
			AlarmUrl   string   `yaml:"alarm-urls"`
			Email      string   `yaml:"email"`
			ApiKey     string   `yaml:"api-key"`
			UserEmails []string `yaml:"user-emails"`
		}
	} `yaml:"zulip-alarm"`

	Redmine struct {
		Enabled  bool   `yaml:"enabled"`
		ApiKey   string `yaml:"api-key"`
		Url      string `yaml:"url"`
		Interval int    `yaml:"interval"`
		Limit    int    `yaml:"limit"`
	} `yaml:"redmine"`

	AutoUpdate struct {
		Enabled  bool `yaml:"enabled"`
		Interval int  `yaml:"interval"`
	} `yaml:"auto-update"`
}

type OsHealthConfigType struct {
	SystemLoadAlarm struct {
		Enabled         bool    `yaml:"enabled"`
		LimitMultiplier float64 `yaml:"limit-multiplier"`

		TopProcesses struct {
			Enabled   bool `yaml:"enabled"`
			Processes int  `yaml:"processes"`
		} `yaml:"top-processes"`
	} `yaml:"system-load-alarm"`

	RamUsageAlarm struct {
		Enabled bool `yaml:"enabled"`
		Limit   int  `yaml:"limit"`

		TopProcesses struct {
			Enabled   bool `yaml:"enabled"`
			Processes int  `yaml:"processes"`
		} `yaml:"top-processes"`
	} `yaml:"ram-usage-alarm"`

	DiskUsageAlarm struct {
		Enabled bool `yaml:"enabled"`
		Limit   int  `yaml:"limit"`
	} `yaml:"disk-usage-alarm"`

	VersionAlarm struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"version-alarm"`

	ServiceHealthAlarm struct {
		Enabled  bool     `yaml:"enabled"`
		Services []string `yaml:"services"`
	} `yaml:"service-health-alarm"`

	PowerAlarm struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"power-alarm"`
}

type UfwApplyConfigType struct {
	RuleSources []struct {
		Url      string `yaml:"url"`
		Protocol string `yaml:"protocol"`
		Port     string `yaml:"port"`
		Comment  string `yaml:"comment"`
	} `yaml:"rule-sources"`
	StaticRules []struct {
		IP       string `yaml:"ip"`
		Protocol string `yaml:"protocol"`
		Port     string `yaml:"port"`
		Comment  string `yaml:"comment"`
	} `yaml:"static-rules"`
	RulesetDir string `yaml:"ruleset-dir"`
}

type DBConfigType struct {
	Mysql struct {
		ProcessLimit int `yaml:"process-limit"`
		Credentials  struct {
			User                 string `yaml:"user"`
			Password             string `yaml:"password"`
			Host                 string `yaml:"host"`
			Port                 int    `yaml:"port"`
			DBName               string `yaml:"dbname"`
			Network              string `yaml:"network"`
			Socket               string `yaml:"socket"`
			AllowNativePasswords bool   `yaml:"allow-native-passwords"`
		} `yaml:"credentials"`

		AutoRepair struct {
			Enabled bool   `yaml:"enabled"`
			Day     string `yaml:"day"`  // Mon Tue Wed Thu Fri Sat Sun
			Hour    string `yaml:"hour"` // 24h format, e.g. 05:00, 17:00
		} `yaml:"auto-repair"`

		// Not implemented yet, but will be used for future cluster monitoring features
		Cluster struct {
			Enabled           bool    `yaml:"enabled"`
			ClusterType       string  `yaml:"cluster-type"` // ndb, xtradb
			Size              int     `yaml:"size"`
			ReceiveQueueLimit int     `yaml:"receive-queue-limit"`
			FlowControlLimit  float64 `yaml:"flow-control-limit"`
		} `yaml:"cluster"`

		Alarm struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"alarm"`
	} `yaml:"mysql"`

	MariaDB struct {
		ProcessLimit int `yaml:"process-limit"`
		Credentials  struct {
			User                 string `yaml:"user"`
			Password             string `yaml:"password"`
			Host                 string `yaml:"host"`
			Port                 int    `yaml:"port"`
			DBName               string `yaml:"dbname"`
			Network              string `yaml:"network"`
			Socket               string `yaml:"socket"`
			AllowNativePasswords bool   `yaml:"allow-native-passwords"`
		} `yaml:"credentials"`

		Cluster struct {
			Enabled           bool    `yaml:"enabled"`
			ClusterType       string  `yaml:"cluster-type"` // galera
			Size              int     `yaml:"size"`
			ReceiveQueueLimit int     `yaml:"receive-queue-limit"`
			FlowControlLimit  float64 `yaml:"flow-control-limit"`
		} `yaml:"cluster"`

		AutoRepair struct {
			Enabled bool   `yaml:"enabled"`
			Day     string `yaml:"day"`  // Mon Tue Wed Thu Fri Sat Sun
			Hour    string `yaml:"hour"` // 24h format, e.g. 05:00, 17:00
		} `yaml:"auto-repair"`

		PMMAgent struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"pmm-agent"`

		Alarm struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"alarm"`
	} `yaml:"mariadb"`

	PostgreSQL struct {
		ProcessLimit           int `yaml:"process-limit"`
		ActiveQueryLimit       int `yaml:"active-query-limit"`
		ConnectionLimitPercent int `yaml:"connection-limit-percent"`

		Credentials struct {
			Mode             string `yaml:"mode"` // "manual" or "string"
			ConnectionString string `yaml:"connection-string"`
			Host             string `yaml:"host"`
			Port             int    `yaml:"port"`
			User             string `yaml:"user"`
			Password         string `yaml:"password"`
			DBName           string `yaml:"dbname"`
		} `yaml:"credentials"`

		LongQuery struct {
			Enabled  bool `yaml:"enabled"`
			Duration int  `yaml:"duration"` // in seconds
		} `yaml:"long-query"`

		PMMAgent struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"pmm-agent"`

		Alarm struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"alarm"`

		// Not implemented yet, can be added in the future versions
		Cluster struct {
			Enabled           bool    `yaml:"enabled"`
			ClusterType       string  `yaml:"cluster-type"` // pgpool, patroni
			Size              int     `yaml:"size"`
			ReceiveQueueLimit int     `yaml:"receive-queue-limit"`
			FlowControlLimit  float64 `yaml:"flow-control-limit"`
		} `yaml:"cluster"`
	} `yaml:"postgresql"`
}
