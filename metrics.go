package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/tedpearson/ForecastMetrics/v3/source"
	"github.com/tedpearson/ForecastMetrics/v3/weather"
)

const ForecastTimeFormat = "2006-01-02:15"

// writes all metrics to VM
type MetricUpdater struct {
	writeApi           api.WriteAPIBlocking
	overwrite          bool
	weatherMeasurement string
	astroMeasurement   string
	astroStateDir      string
}

// takes forecast, location, source
func (m MetricUpdater) WriteMetrics(forecast source.Forecast, location string, src string) {
	forecastOptions := weather.WriteOptions{
		ForecastSource:  src,
		MeasurementName: m.weatherMeasurement,
		Location:        location,
	}
	if !m.overwrite {
		forecastTime := time.Now().Truncate(time.Hour).Format(ForecastTimeFormat)
		forecastOptions.ForecastTime = &forecastTime
	}

	ft := "nil"
	if forecastOptions.ForecastTime != nil {
		ft = *forecastOptions.ForecastTime
	}
	records := forecast.WeatherRecords
	log.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s", forecast_time:"%s"}`,
		len(records), location, src, m.weatherMeasurement, ft)

	points := toPoints(records, forecastOptions)
	if err := m.writeApi.WritePoint(context.Background(), points...); err != nil {
		log.Printf("%+v", err)
	}

	// write next hour to past forecast measurement
	if !m.overwrite {
		nextHour := time.Now().Truncate(time.Hour).Add(time.Hour)
		for _, record := range records {
			if nextHour.Equal(record.Time) {
				nextHourRecord := []source.WeatherRecord{record}
				nextHourOptions := forecastOptions
				f := "0"
				nextHourOptions.ForecastTime = &f
				points = toPoints(nextHourRecord, nextHourOptions)
				if err := m.writeApi.WritePoint(context.Background(), points...); err != nil {
					log.Printf("%+v", err)
				}
				break
			}
		}
	}

	if len(forecast.AstroEvents) > 0 {
		// write astronomy
		astronomyOptions := forecastOptions
		astronomyOptions.MeasurementName = m.astroMeasurement
		astronomyOptions.ForecastTime = nil
		// filter points to only those after last written point
		stateFile := filepath.Join(m.astroStateDir, src, location)
		lastWrittenTime := readState(stateFile)
		lastTimePoint := lastWrittenTime
		eventsToWrite := make([]source.AstroEvent, 0)
		for _, event := range forecast.AstroEvents {
			if event.Time.After(lastWrittenTime) {
				eventsToWrite = append(eventsToWrite, event)
			}
			if event.Time.After(lastTimePoint) {
				lastTimePoint = event.Time
			}
		}
		log.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s"}`,
			len(eventsToWrite), location, src, m.astroMeasurement)
		points := toPoints(eventsToWrite, astronomyOptions)
		if err := m.writeApi.WritePoint(context.Background(), points...); err != nil {
			log.Printf("%+v", err)
			return
		}
	}
}

func readState(stateFile string) time.Time {
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

func toPoints[IP source.InfluxPointer](ip []IP, options weather.WriteOptions) []*write.Point {
	points := make([]*write.Point, 0, len(ip))
	for _, item := range ip {
		t := reflect.ValueOf(item).FieldByName("Time").Interface().(time.Time)
		// only send future datapoints.
		ft := options.ForecastTime
		if ft != nil && *ft != "0" && t.Before(time.Now().Add(time.Hour+1)) {
			continue
		}
		points = append(points, toPoint(t, item, options))
	}
	return points
}

func toPoint(t time.Time, i interface{}, options weather.WriteOptions) *write.Point {
	tags := map[string]string{
		"source":   options.ForecastSource,
		"location": options.Location,
	}
	if options.ForecastTime != nil {
		tags["forecast_time"] = *options.ForecastTime
	}
	fields := make(map[string]interface{})
	e := reflect.ValueOf(i)
	for i := 0; i < e.NumField(); i++ {
		name := strcase.ToSnake(e.Type().Field(i).Name)
		// note: skip time field (added when creating the point)
		if name == "time" {
			continue
		}
		ptr := e.Field(i)
		if ptr.IsNil() {
			// don't dereference nil pointers
			continue
		}
		val := ptr.Elem().Interface()
		fields[name] = val
	}
	return write.NewPoint(options.MeasurementName, tags, fields, t)
}
