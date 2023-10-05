package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	myhttp "github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/source"
)

var (
	version   string = "development"
	goVersion string = "unknown"
	buildDate string = "unknown"
)

func main() {
	// parse flags
	configFile := flag.String("config", "forecastmetrics.yaml", "Config file")
	locationsFile := flag.String("locations", "locations.yaml", "Locations file")
	versionFlag := flag.Bool("v", false, "Show version and exit")
	flag.Parse()
	fmt.Printf("ForecastMetrics version %s built on %s with %s\n", version, buildDate, goVersion)
	if *versionFlag {
		os.Exit(0)
	}
	configService := NewConfigService(*configFile, *locationsFile)
	config := configService.Config
	locationService := LocationService{BingToken: config.BingToken}
	forecasters := MakeForecasters(config.Sources.Enabled, config.HttpCacheDir, config.Sources.VisualCrossing.Key)
	c := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.AuthToken)
	writeApi := c.WriteAPIBlocking(config.InfluxDB.Org, config.InfluxDB.Bucket)
	metricUpdater := MetricUpdater{
		writeApi:           writeApi,
		overwrite:          config.OverwriteData,
		weatherMeasurement: config.ForecastMeasurementName,
		astroMeasurement:   config.AstronomyMeasurementName,
	}
	scheduler := Scheduler{
		ConfigService: configService,
		MetricUpdater: metricUpdater,
		Forecasters:   forecasters,
	}
	scheduler.Start()
	dispatcher := NewDispatcher(forecasters, configService, scheduler, config.AdHocCacheEntries)
	promConverter := PromConverter{
		ForecastMeasurementName:  config.ForecastMeasurementName,
		AstronomyMeasurementName: config.AstronomyMeasurementName,
	}
	server := Server{
		LocationService: locationService,
		Dispatcher:      dispatcher,
		ConfigService:   configService,
		PromConverter:   promConverter,
		AuthToken:       config.InfluxDB.AuthToken,
		ProxyUrl:        config.ProxyUrl,
	}
	server.Start(config.ServerPort)
}

// MakeForecasters creates the forecasters with an exponential backoff retrying http client.
// Only enabled forecasters are returned.
func MakeForecasters(enabled []string, cacheDir string, vcKey string) map[string]source.Forecaster {
	// create retryer
	client := httpcache.NewTransport(diskcache.New(cacheDir)).Client()
	retryer := myhttp.Retryer{
		Client: client,
	}
	forecasters := map[string]source.Forecaster{
		"nws": &source.NWS{
			Retryer: retryer,
		},
		"visualcrossing": &source.VisualCrossing{
			Retryer: retryer,
			Key:     vcKey,
		},
	}
	// only return enabled forecasters
	for name := range forecasters {
		if !slices.Contains(enabled, name) {
			delete(forecasters, name)
		}
	}
	return forecasters
}

// todo
//   grafana dashboards
//     change from locationAdhoc/locationTxt to just location
//     add "save" tag, which determines if it should be added to locations.
//   make influx forwarded token and our required auth token allowed to be different
//   update readme
//   allow http server functionality to be turned off if desired, by not including a port to listen on or something
//   also allow proxy to be turned off
//   possible to implement precip sums? +probability?
