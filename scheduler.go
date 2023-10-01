package main

import (
	"context"
	"fmt"
	"time"

	"github.com/stephenafamo/kronika"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

// Scheduler runs regular exports of forecast metrics to the database.
type Scheduler struct {
	ConfigService ConfigService
	MetricUpdater MetricUpdater
	Forecasters   map[string]source.Forecaster
}

// Start starts the goroutine to run regular exports.
func (s Scheduler) Start() {
	go s.run()
}

// run loops and calls updateForecasts at the top of each hour.
func (s Scheduler) run() {
	firstRun := time.Now().Truncate(time.Hour)
	for _ = range kronika.Every(context.Background(), firstRun, time.Hour) {
		s.updateForecasts()
	}
}

// updateForecasts calls UpdateForecast for every currently exported location.
func (s Scheduler) updateForecasts() {
	// get latest config from config svc
	locations := s.ConfigService.GetLocations()
	// loop through source, locations. call forecast service, metric service.
	for _, location := range locations {
		s.UpdateForecast(location)
	}
}

// UpdateForecast gets the forecast and writes the metrics to the database for every enabled Forecaster.
func (s Scheduler) UpdateForecast(location Location) {
	for src, forecaster := range s.Forecasters {
		fmt.Printf("Getting scheduled forecast for %s from %T\n", location.Name, forecaster)
		forecast, err := forecaster.GetForecast(location.Latitude, location.Longitude)
		if err != nil {
			fmt.Printf("Failed to get forecast for %+v from %s: %v\n", location, src, err)
			continue
		}
		s.MetricUpdater.WriteMetrics(*forecast, location.Name, src)
	}
}
