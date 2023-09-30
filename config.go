package main

import (
	"os"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"
)

type Location struct {
	Name      string
	Latitude  string
	Longitude string
}

type InfluxConfig struct {
	Host      string
	AuthToken string `yaml:"auth_token"`
	Org       string
	Bucket    string
}

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

type ConfigService struct {
	Config     Config
	ConfigFile string
	lock       *sync.Mutex
}

func (c ConfigService) HasLocation(location Location) bool {
	return slices.Contains(c.Config.Locations, location)
}

func (c ConfigService) GetLocations() []Location {
	return c.Config.Locations
}

func (c ConfigService) AddLocation(location Location) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Config.Locations = append(c.Config.Locations, location)
	c.marshall()
}

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
