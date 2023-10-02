package main

import (
	"fmt"
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
	InfluxDB                 InfluxConfig `yaml:"influxdb"`
	ForecastMeasurementName  string       `yaml:"forecast_measurement_name"`
	AstronomyMeasurementName string       `yaml:"astronomy_measurement_name"`
	HttpCacheDir             string       `yaml:"http_cache_dir"`
	OverwriteData            bool         `yaml:"overwrite_data"`
	BingToken                string       `yaml:"bing_token"`
	ServerPort               int64        `yaml:"server_port"`
	AdHocCacheEntries        int          `yaml:"ad_hoc_cache_entries"`
	ProxyUrl                 string       `yaml:"proxy_url"`
	Sources                  struct {
		Enabled        []string
		VisualCrossing struct {
			Key string
		} `yaml:"visualcrossing"`
	}
}

// ConfigService provides a way to update and get the latest list of locations that have regular
// forecasts exported to the database.
type ConfigService struct {
	Config        Config
	locationsFile string
	lock          *sync.Mutex
	locations     []Location
}

// NewConfigService initializes a ConfigService by parsing the main config and the locations files.
// It panics if it can't read or parse the configs.
func NewConfigService(configFile, locationsFile string) *ConfigService {
	// read config
	cf, err := os.ReadFile(configFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading config file %s: %s", configFile, err))
	}
	var config Config
	err = yaml.Unmarshal(cf, &config)
	if err != nil {
		panic(fmt.Sprintf("Error loading config from %s: %s", configFile, err))
	}
	lf, err := os.ReadFile(locationsFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading locations file %s: %s", locationsFile, err))
	}
	var locations []Location
	err = yaml.Unmarshal(lf, &locations)
	if err != nil {
		panic(fmt.Sprintf("Error loading locations from %s: %s", locationsFile, err))
	}
	return &ConfigService{
		Config:        config,
		locationsFile: locationsFile,
		lock:          &sync.Mutex{},
		locations:     locations,
	}
}

// HasLocation returns true if this Location is being regularly exported.
func (c *ConfigService) HasLocation(location Location) bool {
	return slices.Contains(c.locations, location)
}

// GetLocations returns a copy of all actively exported locations.
func (c *ConfigService) GetLocations() []Location {
	c.lock.Lock()
	defer c.lock.Unlock()
	locsCopy := make([]Location, len(c.locations))
	copy(locsCopy, c.locations)
	return locsCopy
}

// AddLocation adds a new location to be regularly exported. It is saved to the config file.
func (c *ConfigService) AddLocation(location Location) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.locations = append(c.locations, location)
	c.marshall()
}

// RemoveLocation removes a location from being regularly exported, and removes it from the config file.
func (c *ConfigService) RemoveLocation(location Location) {
	c.lock.Lock()
	defer c.lock.Unlock()
	locs := c.locations
	idx := slices.Index(locs, location)
	if idx > -1 {
		c.locations = append(locs[:idx], locs[idx+1:]...)
		c.marshall()
	}
}

// marshall writes the current configuration to the config file.
// It should only be called while holding the lock.
func (c *ConfigService) marshall() {
	f, err := os.OpenFile(c.locationsFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Sprintf("Error opening locations file %s: %s", c.locationsFile, err))
	}
	defer f.Close()
	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	err = encoder.Encode(c.locations)
	if err != nil {
		panic(fmt.Sprintf("Error saving locations to %s: %s", c.locationsFile, err))
	}
	err = encoder.Close()
	if err != nil {
		panic(fmt.Sprintf("Error saving locations to %s: %s", c.locationsFile, err))
	}
}
