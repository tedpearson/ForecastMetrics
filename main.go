package main

import (
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/spf13/viper"
	"github.com/tedpearson/weather2influxdb/http"
	"github.com/tedpearson/weather2influxdb/influx"
	"github.com/tedpearson/weather2influxdb/source"
	"github.com/tedpearson/weather2influxdb/weather"
	"log"
	"time"
)

type App struct {
	forecasters  map[string]weather.Forecaster
	forecastTime int64
	writer       influx.Writer
	retryer      http.Retryer
	config       Config
}

func main() {
	viper.SetConfigName("weather2influxdb")
	viper.AddConfigPath("/usr/local/etc")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config file: %+v", err)
	}
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Couldn't decode config: %+v", err)
	}

	client := httpcache.NewTransport(diskcache.New(config.Forecast.HttpCacheDir)).Client()
	//client.Timeout = 2 * time.Second
	retryer := http.Retryer{
		Client: client,
	}
	app := App{
		forecasters:  MakeForecasters(config),
		forecastTime: time.Now().Truncate(time.Hour).Unix() * 1000,
		writer:       influx.New(config.InfluxDB),
		retryer:      retryer,
		config:       config,
	}

	for _, location := range config.Locations {
		for _, src := range config.Forecast.Sources {
			app.RunForecast(src, location)
		}
	}
}

func MakeForecasters(config Config) map[string]weather.Forecaster {
	sources := map[string]weather.Forecaster{
		"nws": source.NWS{},
		"visualcrossing": source.VisualCrossing{
			Key: config.Forecast.VisualCrossing.Key,
		},
	}
	return sources
}

func (app App) RunForecast(src string, location Location) {
	c := app.config
	records, err := app.forecasters[src].GetWeather(location.Latitude, location.Longitude, app.retryer)
	if err != nil {
		log.Printf("%+v", err)
		return
	}
	// write forecast
	err = app.writer.WriteMeasurements(influx.WriteOptions{
		Bucket:          c.InfluxDB.Database,
		ForecastSource:  src,
		MeasurementName: c.Forecast.MeasurementName,
		Location:        location.Name,
	}, records)
	if err != nil {
		log.Printf("%+v", err)
	}
	if c.Forecast.History.Enabled {
		err = app.writer.WriteMeasurements(influx.WriteOptions{
			Bucket:         c.InfluxDB.Database + "/" + c.Forecast.History.RetentionPolicy,
			ForecastSource:  src,
			MeasurementName: c.Forecast.History.MeasurementName,
			Location:        location.Name,
			ForecastTime:    &app.forecastTime,
		}, records)
		if err != nil {
			log.Printf("%+v", err)
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
		Sources        []string
		HttpCacheDir   string `mapstructure:"http_cache_dir"`
		VisualCrossing struct {
			Key string
		} `mapstructure:"visualcrossing"`
	}
}

// todo:
//  visualcrossing (free version for now)
//  theglobalweather (test with free(?), or sign up, cheap per call)
//  authentication as needed
//  add error handling for things like bad response from api, no points receieved, bad data
//  (e.g. massively negative apparent temp on datapoints with no other data)
//  build ci and releases in github actions
//  test coverage
//  embed build version in binary: https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
//  see if vc in metric is any more accurate for precipitation
