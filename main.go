package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	myhttp "github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/source"
)

var (
	version   = "development"
	goVersion = "unknown"
	buildDate = "unknown"
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
	locationService := LocationService{
		AzureSharedKey: config.AzureSharedKey,
		cache:          cache.New(cache.AsLRU[string, LocationResult](lru.WithCapacity(200))),
	}
	forecasters := MakeForecasters(config.Sources.Enabled, config.HttpCacheDir, config.Sources.VisualCrossing.Key)
	c := influxdb2.NewClient(config.InfluxDB.Host, config.InfluxDB.AuthToken)
	writeApi := c.WriteAPIBlocking(config.InfluxDB.Org, config.InfluxDB.Bucket)
	metricUpdater := MetricUpdater{
		writeApi:           writeApi,
		overwrite:          config.OverwriteData,
		weatherMeasurement: config.ForecastMeasurementName,
		astroMeasurement:   config.AstronomyMeasurementName,
		precipProbability:  config.PrecipProbability,
	}
	scheduler := Scheduler{
		ConfigService: configService,
		MetricUpdater: metricUpdater,
		Forecasters:   forecasters,
	}
	scheduler.Start()
	if config.ServerConfig.Port == 0 {
		// no port specified, keep other goroutines running
		runtime.Goexit()
	} else {
		// only start server if port is specified
		dispatcher := NewDispatcher(forecasters, configService, scheduler, config.AdHocCacheEntries)
		promConverter := PromConverter{
			ForecastMeasurementName:  config.ForecastMeasurementName,
			AstronomyMeasurementName: config.AstronomyMeasurementName,
			PrecipProbability:        config.PrecipProbability,
		}
		server := Server{
			LocationService: locationService,
			Dispatcher:      dispatcher,
			PromConverter:   promConverter,
			AuthToken:       config.InfluxDB.AuthToken,
			AllowedMetricNames: []string{
				config.ForecastMeasurementName,
				config.AstronomyMeasurementName,
				"accumulated_precip",
			},
		}
		server.Start(config.ServerConfig)
	}
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
