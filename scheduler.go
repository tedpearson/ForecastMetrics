package main

import (
	"context"
	"fmt"
	"time"

	"github.com/stephenafamo/kronika"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

type Scheduler struct {
	ConfigService ConfigService
	MetricUpdater MetricUpdater
	Forecasters   map[string]source.ForecasterV2
}

// runs a forecast update for all configured locations/sources every hour
// gets latest configuration each time

func (s Scheduler) Start() {
	go s.run()
}

func (s Scheduler) run() {
	firstRun := time.Now().Truncate(time.Hour)
	for _ = range kronika.Every(context.Background(), firstRun, time.Hour) {
		s.updateForecasts()
	}
}

func (s Scheduler) updateForecasts() {
	// get latest config from config svc
	locations := s.ConfigService.GetLocations()
	// loop through source, locations. call forecast service, metric service.
	for _, location := range locations {
		s.UpdateForecast(location)
	}
}

func (s Scheduler) UpdateForecast(location Location) {
	for src, forecaster := range s.Forecasters {
		forecast, err := forecaster.GetForecast(location.Latitude, location.Longitude)
		if err != nil {
			fmt.Printf("Failed to get forecast for %+v from %s: %v", location, src, err)
			continue
		}
		s.MetricUpdater.WriteMetrics(*forecast, location.Name, src)
	}
}

// create scheduled forecast updater
//   needs config service
//   needs metric updater service
//   needs forecast request processor
