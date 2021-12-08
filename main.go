package main

import (
	"log"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/spf13/viper"
	"github.com/tedpearson/weather2influxdb/http"
	"github.com/tedpearson/weather2influxdb/influx"
	"github.com/tedpearson/weather2influxdb/source"
	"github.com/tedpearson/weather2influxdb/weather"
)

type App struct {
	forecasters  map[string]weather.Forecaster
	forecastTime string
	writer       *influx.Writer
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

	client := httpcache.NewTransport(diskcache.New(config.HttpCacheDir)).Client()
	//client.Timeout = 2 * time.Second
	retryer := http.Retryer{
		Client: client,
	}
	writer, err := influx.New(config.InfluxDB)
	if err != nil {
		log.Fatalf("Couldn't connect to influx: %+v", err)
	}
	app := App{
		forecasters:  MakeForecasters(config),
		forecastTime: time.Now().Format("2006-01-02-15"),
		writer:       writer,
		retryer:      retryer,
		config:       config,
	}

	for _, location := range config.Locations {
		for _, src := range config.Sources.Enabled {
			app.RunForecast(src, location)
		}
	}
}

func MakeForecasters(config Config) map[string]weather.Forecaster {
	sources := map[string]weather.Forecaster{
		"nws": &source.NWS{},
		"visualcrossing": &source.VisualCrossing{
			Key: config.Sources.VisualCrossing.Key,
		},
		"theglobalweather": &source.TheGlobalWeather{
			Key: config.Sources.TheGlobalWeather.Key,
		},
	}
	return sources
}

func (app App) RunForecast(src string, loc Location) {
	c := app.config
	forecaster := app.forecasters[src]
	err := forecaster.Init(loc.Latitude, loc.Longitude, app.retryer)
	if err != nil {
		log.Printf("%+v", err)
		return
	}
	records, err := app.forecasters[src].GetWeather()
	if err != nil {
		log.Printf("%+v", err)
		return
	}
	forecastOptions := weather.WriteOptions{
		ForecastSource:  src,
		MeasurementName: c.Forecast.MeasurementName,
		Location:        loc.Name,
		ForecastTime:    app.forecastTime,
		Database:        c.InfluxDB.Database,
	}

	// write forecast
	log.Printf(`Writing %d points to "%s" in InfluxDB for "%s"`, len(records.Values), c.Forecast.MeasurementName, src)
	err = app.writer.WriteMeasurements(
		records.ToPoints(forecastOptions))
	if err != nil {
		log.Printf("%+v", err)
	}
	// write astronomy
	astronomyOptions := forecastOptions
	astronomyOptions.MeasurementName = c.Astronomy.MeasurementName
	if c.Astronomy.Enabled {
		app.RunAstrocast(forecaster, astronomyOptions)
	}

}

func (app App) RunAstrocast(forecaster weather.Forecaster, options weather.WriteOptions) {
	astrocaster, ok := forecaster.(weather.Astrocaster)
	if ok {
		events, err := astrocaster.GetAstrocast()
		if err != nil {
			log.Printf("%+v", err)
			return
		}
		log.Printf(`Writing %d points to "%s" in InfluxDB for "%s"`, len(events.Values), options.MeasurementName,
			options.ForecastSource)
		err = app.writer.WriteMeasurements(events.ToPoints(options))
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
	}
	Astronomy struct {
		Enabled         bool
		MeasurementName string `mapstructure:"measurement_name"`
	}
	Sources struct {
		Enabled        []string
		VisualCrossing struct {
			Key string
		} `mapstructure:"visualcrossing"`
		TheGlobalWeather struct {
			Key string
		} `mapstructure:"theglobalweather"`
	}
	HttpCacheDir string `mapstructure:"http_cache_dir"`
}

// todo:
//  add error handling for things like bad response from api, no points receieved, bad data
//  (e.g. massively negative apparent temp on datapoints with no other data)
//  build ci and releases in github actions
//  test coverage
//  embed build version in binary: https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
//  see if vc in metric is any more accurate for precipitation
//  add code documentation
//  update readme with how to set up and use (config, influx, accounts)
