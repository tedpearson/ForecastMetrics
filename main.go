package main

import (
	"github.com/go-errors/errors"
	"github.com/spf13/viper"
	"github.com/tedpearson/weather2influxdb/influx"
	"github.com/tedpearson/weather2influxdb/nws"
	"github.com/tedpearson/weather2influxdb/weather"
	"log"
	"time"
)

var (
	// Add new Forecasters here so they are supported in the config
	// alternatively, make each forecaster register with a registry.
	sources = map[string]weather.Forecaster{
		"nws": nws.NWS{},
	}
	// if this ever becomes a long running process, move this into RunForecast().
	forecastTime = time.Now().Truncate(time.Minute).Unix() * 1000
)

func main() {
	viper.SetConfigName("weather2influxdb")
	viper.AddConfigPath("/usr/local/etc")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	handleError(err)
	var config Config
	err = viper.Unmarshal(&config)
	handleError(err)

	writer := influx.New(config.InfluxDB)

	for _, location := range config.Locations {
		for _, source := range config.Forecast.Sources {
			RunForecast(config, source, location, writer)
		}
	}
}

func RunForecast(config Config, source string, location Location, writer influx.Writer) {
	records, err := sources[source].GetWeather(location.Latitude, location.Longitude, config.Forecast.HttpCacheDir)
	handleError(err)
	// write forecast
	err = writer.WriteMeasurements(influx.WriteOptions{
		Bucket:          config.InfluxDB.Database,
		ForecastSource:  source,
		MeasurementName: config.Forecast.MeasurementName,
		Location:        location.Name,
		ForecastTime:    forecastTime,
	}, records)
	handleError(err)
	if config.Forecast.History.Enabled {
		err = writer.WriteMeasurements(influx.WriteOptions{
			Bucket:          config.InfluxDB.Database + "/" + config.Forecast.History.RetentionPolicy,
			ForecastSource:  source,
			MeasurementName: config.Forecast.History.MeasurementName,
			Location:        location.Name,
			ForecastTime:    forecastTime,
		}, records)
		handleError(err)
	}
}

func handleError(err error) {
	if err != nil {
		switch err.(type) {
		case *errors.Error:
			// todo: only call ErrorStack if it's an errors error. Look at docs.
			log.Fatalf("Error, exiting: %v", err.(*errors.Error).ErrorStack())
		default:
			log.Fatalf("Error, exiting: %v", err)
		}
	}
}

type Location struct {
	Name      string
	Latitude  string
	Longitude string
}

type Config struct {
	Locations []Location
	InfluxDB  influx.Config
	Forecast  struct {
		MeasurementName string `mapstructure:"measurement_name"`
		History         struct {
			Enabled         bool
			RetentionPolicy string `mapstructure:"retention_policy"`
			MeasurementName string `mapstructure:"measurement_name"`
		}
		Sources      []string
		HttpCacheDir string `mapstructure:"http_cache_dir"`
	}
}

// todo:
//  visualcrossing (free version for now)
//  theglobalweather (test with free(?), or sign up, cheap per call)
//  authentication as needed
//  add error handling for things like bad response from api, no points receieved, bad data
//   specifically, figure out why sometimes there are zero points from nws
//  (e.g. massively negative apparent temp on datapoints with no other data)
//  build ci and releases in github actions
//  test coverage
