package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	myhttp "github.com/tedpearson/ForecastMetrics/http"
	"github.com/tedpearson/ForecastMetrics/source"
	"github.com/tedpearson/ForecastMetrics/weather"
)

type Location struct {
	Name      string
	Latitude  string
	Longitude string
}

type Influx struct {
	Host      string
	AuthToken string `mapstructure:"auth_token"`
	Org       string
	Bucket    string
}

type Config struct {
	Locations []Location
	InfluxDB  Influx
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
	HttpCacheDir  string `mapstructure:"http_cache_dir"`
	StateDir      string `mapstructure:"state_dir"`
	OverwriteData bool   `mapstructure:"overwrite_data"`
}

type App struct {
	forecasters map[string]weather.Forecaster
	writeApi    api.WriteAPIBlocking
	retryer     myhttp.Retryer
	config      Config
}

func main() {
	viper.SetConfigName("forecastmetrics")
	viper.AddConfigPath("/etc")
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
	retryer := myhttp.Retryer{
		Client: client,
	}

	ic := config.InfluxDB
	c := influxdb2.NewClient(ic.Host, ic.AuthToken)
	writeApi := c.WriteAPIBlocking(ic.Org, ic.Bucket)

	app := App{
		forecasters: MakeForecasters(config),
		writeApi:    writeApi,
		retryer:     retryer,
		config:      config,
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
	}
	if !c.OverwriteData {
		forecastTime := time.Now().Truncate(time.Hour).Format("2006-01-02:15")
		forecastOptions.ForecastTime = &forecastTime
	}

	// write forecast

	log.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s", forecast_time:"%s"}`,
		len(records), loc.Name, src, c.Forecast.MeasurementName, *forecastOptions.ForecastTime)

	points := weather.RecordsToPoints(records, forecastOptions)
	if err = app.writeApi.WritePoint(context.Background(), points...); err != nil {
		log.Printf("%+v", err)
	}
	// write next hour to past forecast measurement
	if !c.OverwriteData {
		nextHour := time.Now().Truncate(time.Hour).Add(time.Hour)
		for _, record := range records {
			if nextHour.Equal(record.Time) {
				nextHourRecord := []weather.Record{record}
				nextHourOptions := forecastOptions
				f := "0"
				nextHourOptions.ForecastTime = &f
				points = weather.RecordsToPoints(nextHourRecord, nextHourOptions)
				if err = app.writeApi.WritePoint(context.Background(), points...); err != nil {
					log.Printf("%+v", err)
				}
				break
			}
		}
	}
	// write astronomy
	astronomyOptions := forecastOptions
	astronomyOptions.MeasurementName = c.Astronomy.MeasurementName
	astronomyOptions.ForecastTime = nil
	if c.Astronomy.Enabled {
		app.RunAstrocast(forecaster, astronomyOptions)
	}
}

func (app App) RunAstrocast(forecaster weather.Forecaster, options weather.WriteOptions) {
	astrocaster, ok := forecaster.(weather.Astrocaster)
	if !ok {
		return
	}
	events, err := astrocaster.GetAstrocast()
	if err != nil {
		log.Printf("%+v", err)
		return
	}
	// filter points to only those after last written point
	stateFile := filepath.Join(app.config.StateDir, options.ForecastSource, options.Location)
	lastWrittenTime := ReadState(stateFile)
	lastTimePoint := lastWrittenTime
	eventsToWrite := make([]weather.AstroEvent, 0)
	for _, event := range events {
		if event.Time.After(lastWrittenTime) {
			eventsToWrite = append(eventsToWrite, event)
		}
		if event.Time.After(lastTimePoint) {
			lastTimePoint = event.Time
		}
	}
	log.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s"}`,
		len(eventsToWrite), options.Location, options.ForecastSource, options.MeasurementName)
	points := weather.AstroToPoints(eventsToWrite, options)
	err = app.writeApi.WritePoint(context.Background(), points...)
	if err != nil {
		log.Printf("%+v", err)
		return
	}
	// save lastTimePoint
	WriteState(stateFile, lastTimePoint)
}

func WriteMeasurements(apiWrite api.WriteAPIBlocking, points []*write.Point, err error) error {
	if err != nil {
		return err
	}
	err = apiWrite.WritePoint(context.Background(), points...)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func ReadState(stateFile string) time.Time {
	state, err := os.ReadFile(stateFile)
	lastWrittenTime := time.Now()
	if err != nil {
		log.Printf("Failed to load state: %+v", err)
	} else {
		err = json.Unmarshal(state, &lastWrittenTime)
		if err != nil {
			log.Printf("Failed to unmarshal state: %+v", err)
		}
	}
	return lastWrittenTime
}

func WriteState(stateFile string, time time.Time) {
	newState, err := json.Marshal(time)
	if err != nil {
		log.Printf("Failed to marshal state: %+v", err)
		return
	}
	dir := filepath.Dir(stateFile)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Printf("Failed to create state file dir: %+v", err)
		return
	}
	err = os.WriteFile(stateFile, newState, 0644)
	if err != nil {
		log.Printf("Failed to write state: %+v", err)
	}
}

// todo:
//  add error handling for things like bad response from api, no points receieved, bad data
//  (e.g. massively negative apparent temp on datapoints with no other data)
//  test coverage
//  embed build version in binary: https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
//  add code documentation
