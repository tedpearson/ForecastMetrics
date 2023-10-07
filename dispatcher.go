package main

import (
	"fmt"
	"slices"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

// Dispatcher handles ad-hoc forecast requests, allowing multiple requests for a given
// Location simultaneously, while only running one forecast thread per location at a time.
type Dispatcher struct {
	cache         *cache.Cache[CacheKey, Reply]
	forecasters   map[string]source.Forecaster
	scheduler     Scheduler
	configService *ConfigService
	requests      chan Request
	results       chan Result
	awaiting      map[CacheKey]*[]Request
}

// CacheKey represents the key used in the request cache
type CacheKey struct {
	Location Location
	Source   string
}

// Request represents a call to Dispatcher.GetForecast
type Request struct {
	CacheKey
	AdHoc bool
	Reply chan Reply
}

// Result represents a result from a Forecaster
type Result struct {
	CacheKey
	Reply Reply
}

// Reply represents a reply to a call to Dispatcher.GetForecast
type Reply struct {
	Forecast *source.Forecast
	Error    error
}

// NewDispatcher creates a dispatcher, creating the internal channels and cache needed for operation.
// It also starts the dispatcher goroutine.
func NewDispatcher(forecasters map[string]source.Forecaster, configService *ConfigService, scheduler Scheduler, cacheCapacity int) *Dispatcher {
	d := &Dispatcher{
		cache:         cache.New(cache.AsLRU[CacheKey, Reply](lru.WithCapacity(cacheCapacity))),
		forecasters:   forecasters,
		scheduler:     scheduler,
		configService: configService,
		requests:      make(chan Request, 10),
		results:       make(chan Result, 10),
		awaiting:      make(map[CacheKey]*[]Request),
	}
	go d.runLoop()
	return d
}

// runLoop is where forecast requests and replies are handled so that only one forecast per
// Location is running at once.
func (d *Dispatcher) runLoop() {
	for {
		select {
		case req := <-d.requests:
			// check cache and return if cached
			if value, ok := d.cache.Get(req.CacheKey); ok {
				req.Reply <- value
				continue
			}
			// check if we are already making a request for this location and if so, add to awaiting
			if chans, ok := d.awaiting[req.CacheKey]; ok {
				*chans = append(*chans, req)
			} else {
				// if not already making request, spawn a new goroutine to make the request and return the result
				d.awaiting[req.CacheKey] = &[]Request{req}
				go d.forwardRequest(req.CacheKey)
			}
		case result := <-d.results:
			// if newly registered location, async (update metrics, update configuration)
			awaiting := *d.awaiting[result.CacheKey]
			delete(d.awaiting, result.CacheKey)
			adhoc := slices.ContainsFunc(awaiting, func(r Request) bool {
				return !r.AdHoc
			})
			// don't allow schedule updates if no name specified
			if adhoc && result.Location.Name != "" {
				go d.addScheduledLocation(result.Location)
			}
			// update cache (for both adhoc and registered, there might be another request before the update config finishes)
			d.cache.Set(result.CacheKey, result.Reply, cache.WithExpiration(time.Hour))
			// return result
			for _, a := range awaiting {
				a.Reply <- result.Reply
			}
		}
	}
}

// forwardRequest gets the forecast from a forecaster and puts the response on the results channel for the run loop.
func (d *Dispatcher) forwardRequest(key CacheKey) {
	if forecaster, ok := d.forecasters[key.Source]; ok {
		fmt.Printf("Getting ad-hoc forecast for %s from %T\n", key.Location.Name, forecaster)
		forecast, err := forecaster.GetForecast(key.Location.Latitude, key.Location.Longitude)
		d.results <- Result{
			CacheKey: key,
			Reply: Reply{
				Forecast: forecast,
				Error:    err,
			},
		}
	} else {
		d.results <- Result{
			CacheKey: key,
			Reply: Reply{
				Error: fmt.Errorf("unable to find forecast source %s", key.Source),
			},
		}
	}
}

// GetForecast requests a forecast, placing the request on the requests channel for the run loop.
func (d *Dispatcher) GetForecast(location Location, source string, adHoc bool) (*source.Forecast, error) {
	// send messages around
	reply := make(chan Reply)
	d.requests <- Request{
		CacheKey: CacheKey{
			Location: location,
			Source:   source,
		},
		AdHoc: adHoc,
		Reply: reply,
	}
	// wait for reply back and send back msg.
	r := <-reply
	return r.Forecast, r.Error
}

// addScheduledLocation populates the database with the first forecast for this location,
// then adds the location to the config.
func (d *Dispatcher) addScheduledLocation(location Location) {
	fmt.Printf("Adding %s to regularly updated locations in config\n", location.Name)
	d.scheduler.UpdateForecast(location)
	d.configService.AddLocation(location)
}
