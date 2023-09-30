package main

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

const ForecastTimeFormat = "2006-01-02:15"

type WriteOptions struct {
	ForecastSource  string
	MeasurementName string
	Location        string
	ForecastTime    *string
}

// writes all metrics to VM
type MetricUpdater struct {
	writeApi           api.WriteAPIBlocking
	overwrite          bool
	weatherMeasurement string
	astroMeasurement   string
}

// takes forecast, location, source
func (m MetricUpdater) WriteMetrics(forecast source.Forecast, location string, src string) {
	forecastOptions := WriteOptions{
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
	fmt.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s", forecast_time:"%s"}`+"\n",
		len(records), location, src, m.weatherMeasurement, ft)

	points := toPoints(records, forecastOptions)
	if err := m.writeApi.WritePoint(context.Background(), points...); err != nil {
		fmt.Printf("%+v\n", err)
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
					fmt.Printf("%+v\n", err)
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
		fmt.Printf(`Writing %d points {loc:"%s", src:"%s", measurement:"%s"}`+"\n",
			len(forecast.AstroEvents), location, src, m.astroMeasurement)
		points := toPoints(forecast.AstroEvents, astronomyOptions)
		if err := m.writeApi.WritePoint(context.Background(), points...); err != nil {
			fmt.Printf("%+v\n", err)
			return
		}
	}
}

func toPoints[IP source.InfluxPointer](ip []IP, options WriteOptions) []*write.Point {
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

func toPoint(t time.Time, i interface{}, options WriteOptions) *write.Point {
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
