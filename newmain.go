package main

import (
	"flag"
	"sync"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	myhttp "github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/source"
)

func main() {
	// separate out metric updating
	// separate out forecast request processing

	// parse flags
	configFile := flag.String("config", "forecastmetrics.yaml", "Config file")
	flag.Parse()
	// parse config
	config := mustParseConfig(*configFile)
	// create name processor
	locationService := LocationService{BingToken: config.BingToken}

	// ✅create config service
	//   ✅needs config
	configService := ConfigService{
		Config:     config,
		ConfigFile: *configFile,
		lock:       &sync.Mutex{},
	}
	// ✅create Forecasters
	forecasters := MakeForecastersV2(config.HttpCacheDir, config.Sources.VisualCrossing.Key)
	// ✅create metric updater service
	c := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.AuthToken)
	writeApi := c.WriteAPIBlocking(config.InfluxDB.Org, config.InfluxDB.Bucket)
	metricUpdater := MetricUpdater{
		writeApi:           writeApi,
		overwrite:          config.OverwriteData,
		weatherMeasurement: config.Forecast.MeasurementName,
		astroMeasurement:   config.Astronomy.MeasurementName,
		astroStateDir:      config.StateDir,
	}
	// ✅create scheduled forecast updater
	//   ✅needs config service
	//   ✅needs metric updater service
	//   ✅needs forecast request processor
	scheduler := Scheduler{
		ConfigService: configService,
		MetricUpdater: metricUpdater,
		Forecasters:   forecasters,
	}
	scheduler.Start()
	// create forecast dispatcher
	//   ✅creates cache
	//   ✅needs forecasters
	//   needs metric updater service
	//   ✅needs config service
	dispatcher := NewDispatcher(forecasters, configService, scheduler, config.AdHocCacheEntries)
	// ✅create http handler
	//   creates prometheus converter
	//   ✅needs name processor
	//   ✅needs forecast dispatcher
	server := Server{
		LocationService: locationService,
		Dispatcher:      dispatcher,
		ConfigService:   configService,
		AuthToken:       config.InfluxDB.AuthToken,
	}
	server.Start(config.ServerPort)
}

func MakeForecastersV2(cacheDir string, vcKey string) map[string]source.ForecasterV2 {
	// create retryer
	client := httpcache.NewTransport(diskcache.New(cacheDir)).Client()
	//client.Timeout = 2 * time.Second
	retryer := myhttp.Retryer{
		Client: client,
	}
	return map[string]source.ForecasterV2{
		"nws": &source.NWS{
			Retryer: retryer,
		},
		"visualcrossing": &source.VisualCrossing{
			Retryer: retryer,
			Key:     vcKey,
		},
	}
}

// todo
//  x documentation
//  x don't hard code metric names
//  x write state file for astrocast again
//   improve logging
//   consider caching situation (http, dispatcher)
//  x remove old code
//  x rename ForecastersV2 to Forecasters
//  x deployment stuff
//   read all code and improve things like err handling.
//   make influx forwarded token and our required auth token allowed to be different
