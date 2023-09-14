package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

type Prometheus struct {
	LocationService LocationService
}

// take start, stop, step, and a max points config
// take forecast, metric name,
// come up with a bunch of timestamps from that
func GetTimestamps(start, end, step int64) []int64 {
	points := 1 + (end-start)/step
	timestamps := make([]int64, points)
	for i := range timestamps {
		timestamps[i] = start
		start += step
	}
	return timestamps
}

// fixme: don't hard code metric names
var queryRE = regexp.MustCompile(`((?:forecast2_|astronomy_)\w+)\{(.+)\}`)
var tagRE = regexp.MustCompile(`(\w+)="([^"]+)"`)

type ParsedQuery struct {
	Metric   string
	Location Location
	Source   string
	AdHoc    bool
}

// todo: probably move this to server?
func (p Prometheus) ParseQuery(query string) (*ParsedQuery, error) {
	matches := queryRE.FindStringSubmatch(query)
	if len(matches) == 0 {
		return nil, errors.New("no matches found")
	}
	pq := &ParsedQuery{
		Metric: matches[1],
	}
	// fixme: this doesn't work, won't match locs with comma.
	tags := strings.Split(matches[2], ",")
	for _, tag := range tags {
		matches = tagRE.FindStringSubmatch(tag)
		switch matches[1] {
		case "locationAdhoc":
			pq.AdHoc = true
			fallthrough
		case "locationTxt":
			location, err := p.LocationService.ParseLocation(matches[2])
			if err != nil {
				return nil, err
			}
			pq.Location = *location
		case "source":
			pq.Source = matches[2]
			continue
		default:
			// unknown tag, ignore.
			continue
		}
	}
	return pq, nil
}

type PromResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}

type PromResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []PromResult `json:"result"`
	} `json:"data"`
}

// a method to match timestamps with forecast data
// get value from most recent hour, otherwise no data, so remove that timestamp?
// todo rename this something good

func ConvertToTimeSeries(forecast source.Forecast, metric string, start, end, step int64) PromResponse {
	pr := PromResponse{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     []PromResult `json:"result"`
		}{
			ResultType: "matrix",
		},
	}
	points := GetMetric(forecast, metric)
	// get timestamps
	// for each timestamp, find equal point or if any point came before it by no more than 1 hour
	// if not, discard timestamp
	i := 0
	timestamps := GetTimestamps(start, end, step)
	values := make([][]any, 0, len(timestamps))
	for _, ts := range timestamps {
		for j, p := range points[i:] {
			if p.Timestamp > ts {
				i = j
				break
			}
			if p.Timestamp == ts || (p.Timestamp < ts && p.Timestamp+3600 > ts) {
				i = j
				v := []any{ts, fmt.Sprintf("%f", p.Metric)}
				values = append(values, v)
				break
			}
		}
	}
	// fixme: pass source/location
	pr.Data.Result = []PromResult{{
		Metric: map[string]string{
			"__name__": metric,
			"source":   "idk",
			"location": "idk",
		},
		Values: values,
	}}
	return pr
}

type Metric struct {
	Timestamp int64
	Metric    float64
}

func GetMetric(forecast source.Forecast, metric string) []Metric {
	parts := strings.SplitN(metric, "_", 2)
	name := strcase.ToCamel(parts[1])
	if parts[0] == "astronomy" {
		points := make([]Metric, len(forecast.AstroEvents))
		for i, record := range forecast.AstroEvents {
			field := reflect.ValueOf(record).FieldByName(name)
			if !field.IsValid() {
				return nil
			}
			points[i] = Metric{
				Timestamp: record.Time.Unix(),
				Metric:    field.Elem().Interface().(float64),
			}
		}
		return points
	} else if parts[0] == "forecast2" {
		points := make([]Metric, len(forecast.WeatherRecords))
		for i, record := range forecast.WeatherRecords {
			field := reflect.ValueOf(record).FieldByName(name)
			if !field.IsValid() {
				return nil
			}
			if field.IsNil() {
				continue
			}
			points[i] = Metric{
				Timestamp: record.Time.Unix(),
				Metric:    field.Elem().Interface().(float64),
			}
		}
		return points
	}
	return nil
}
