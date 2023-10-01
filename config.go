package main

import (
	"os"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"
)

// Location is a name plus geo coordinates.
type Location struct {
	Name      string
	Latitude  string
	Longitude string
}

// InfluxConfig is the configuration for Influx/VictoriaMetrics.
type InfluxConfig struct {
	Host      string
	AuthToken string `yaml:"auth_token"`
	Org       string
	Bucket    string
}

// Config is the configuration for ForecastMetrics.
type Config struct {
	Locations []Location
	InfluxDB  InfluxConfig `yaml:"influxdb"`
	Forecast  struct {
		MeasurementName string `yaml:"measurement_name"`
	}
	Astronomy struct {
		MeasurementName string `yaml:"measurement_name"`
	}
	Sources struct {
		Enabled        []string
		VisualCrossing struct {
			Key string
		} `yaml:"visualcrossing"`
	}
	HttpCacheDir      string `yaml:"http_cache_dir"`
	OverwriteData     bool   `yaml:"overwrite_data"`
	BingToken         string `yaml:"bing_token"`
	ServerPort        int64  `yaml:"server_port"`
	AdHocCacheEntries int    `yaml:"ad_hoc_cache_entries"`
}

// mustParseConfig parses the config from the given config file, or panics.
func mustParseConfig(configFile string) Config {
	// read config
	file, err := os.ReadFile(configFile)
	if err != nil {
		panic(err)
	}
	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic(err)
	}
	return config
}

// ConfigService provides a way to update and get the latest list of locations that have regular
// forecasts exported to the database.
type ConfigService struct {
	Config     Config
	ConfigFile string
	lock       *sync.Mutex
}

// HasLocation returns true if this Location is being regularly exported.
func (c ConfigService) HasLocation(location Location) bool {
	return slices.Contains(c.Config.Locations, location)
}

// GetLocations returns all actively exported locations.
func (c ConfigService) GetLocations() []Location {
	return c.Config.Locations
}

// AddLocation adds a new location to be regularly exported. It is saved to the config file.
func (c ConfigService) AddLocation(location Location) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Config.Locations = append(c.Config.Locations, location)
	c.marshall()
}

// RemoveLocation removes a location from being regularly exported, and removes it from the config file.
func (c ConfigService) RemoveLocation(location Location) {
	c.lock.Lock()
	defer c.lock.Unlock()
	locs := c.Config.Locations
	idx := slices.Index(locs, location)
	if idx > -1 {
		c.Config.Locations = append(locs[:idx], locs[idx+1:]...)
		c.marshall()
	}
}

// marshall writes the current configuration to the config file.
func (c ConfigService) marshall() {
	bytes, err := yaml.Marshal(c.Config)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(c.ConfigFile, bytes, 0644)
	if err != nil {
		panic(err)
	}
}
