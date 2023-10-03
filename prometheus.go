package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"

	"github.com/tedpearson/ForecastMetrics/v3/source"
)

// GetTimestamps generates a slice of epoch time based timestamps.
func GetTimestamps(start, end, step int64) []int64 {
	points := 1 + (end-start)/step
	timestamps := make([]int64, points)
	for i := range timestamps {
		timestamps[i] = start
		start += step
	}
	return timestamps
}

type Metric struct {
	Timestamp int64
	Metric    float64
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

type PromConverter struct {
	ForecastMeasurementName  string
	AstronomyMeasurementName string
}

// ConvertToTimeSeries converts a source.Forecast to a PromResponse which can be
// marshalled to json. It gets the closest corresponding real point in the last hour
// before the timestamp, otherwise that timestamp is dropped.
func (pc PromConverter) ConvertToTimeSeries(forecast source.Forecast, params Params) PromResponse {
	pr := PromResponse{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     []PromResult `json:"result"`
		}{
			ResultType: "matrix",
		},
	}
	points := pc.GetMetric(forecast, params.Metric)
	// get timestamps
	// for each timestamp, find equal point or if any point came before it by no more than 1 hour
	// if not, discard timestamp
	i := 0
	timestamps := GetTimestamps(params.Start, params.End, params.Step)
	values := make([][]any, 0, len(timestamps))
	for _, ts := range timestamps {
		for j, p := range points[i:] {
			if p.Timestamp > ts {
				i = j
				break
			}
			if p.Timestamp == ts || (p.Timestamp < ts && p.Timestamp+params.Step > ts) {
				i = j
				v := []any{ts, fmt.Sprintf("%f", p.Metric)}
				values = append(values, v)
				break
			}
		}
	}
	pr.Data.Result = []PromResult{{
		Metric: map[string]string{
			"__name__": params.Metric,
			"source":   params.Source,
			"location": params.Location.Name,
		},
		Values: values,
	}}
	return pr
}

// GetMetric fetches a single field from each forecast point in the format
// that prometheus uses for output.
func (pc PromConverter) GetMetric(forecast source.Forecast, metric string) []Metric {
	parts := strings.SplitN(metric, "_", 2)
	name := strcase.ToCamel(parts[1])
	if parts[0] == pc.AstronomyMeasurementName {
		points := make([]Metric, 0, len(forecast.AstroEvents))
		for _, record := range forecast.AstroEvents {
			field := reflect.ValueOf(record).FieldByName(name)
			if !field.IsValid() {
				return nil
			}
			if !field.Elem().IsValid() {
				continue
			}
			points = append(points, Metric{
				Timestamp: record.Time.Unix(),
				Metric:    ValueToFloat(field.Elem()),
			})
		}
		return points
	} else if parts[0] == pc.ForecastMeasurementName {
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
				Metric:    ValueToFloat(field.Elem()),
			}
		}
		return points
	}
	return nil
}

func ValueToFloat(value reflect.Value) float64 {
	if value.CanInt() {
		return float64(value.Int())
	}
	if value.CanFloat() {
		return value.Float()
	}
	return 0
}
