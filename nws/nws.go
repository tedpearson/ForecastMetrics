package nws

import (
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v3"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
	"github.com/rickb777/date/period"
	"github.com/tedpearson/weather2influxdb/convert"
	"github.com/tedpearson/weather2influxdb/http"
	"github.com/tedpearson/weather2influxdb/weather"
	"io"
	"log"
	"math"
	"strings"
	"time"
)

type NWS struct{}

func (n NWS) GetWeather(lat string, lon string, cachePath string) ([]weather.Record, error) {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	cache := diskcache.New(cachePath)
	client := httpcache.NewTransport(cache).Client()

	//client := httpcache.NewMemoryCacheTransport().Client()
	log.Println("Looking up NWS location")

	off := backoff.NewExponentialBackOff()
	off.MaxElapsedTime = 22 * time.Second
	body1, err := http.RetryRequest(url, client, off)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer cleanup(body1)
	var jsonResponse map[string]interface{}
	err = json.NewDecoder(body1).Decode(&jsonResponse)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)
	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	log.Println("Getting NWS forecast")
	body2, err := http.RetryRequest(gridpointUrl, client, off)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer cleanup(body2)

	var forecast nwsForecast
	err = json.NewDecoder(body2).Decode(&forecast)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	records, err := transformForecast(forecast)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return records, nil
}

func transformForecast(forecast nwsForecast) ([]weather.Record, error) {
	props := forecast.Properties
	var table = []transformation{
		{
			measurements: props.Temperature,
			setter:       weather.SetTemperature,
			conversion:   convert.CToF,
		},
		{
			measurements: props.Dewpoint,
			setter:       weather.SetDewpoint,
			conversion:   convert.CToF,
		},
		{
			measurements: props.ApparentTemperature,
			setter:       weather.SetFeelsLike,
			conversion:   convert.CToF,
		},
		{
			measurements: props.SkyCover,
			setter:       weather.SetSkyCover,
			conversion:   convert.PercentToRatio,
		},
		{
			measurements: props.WindDirection,
			setter:       weather.SetWindDirection,
			conversion:   convert.Identity,
		},
		{
			measurements: props.WindSpeed,
			setter:       weather.SetWindSpeed,
			conversion:   convert.KmhToMph,
		},
		{
			measurements: props.WindGust,
			setter:       weather.SetWindGust,
			conversion:   convert.KmhToMph,
		},
		{
			measurements: props.ProbabilityOfPrecipitation,
			setter:       weather.SetPrecipitationProbability,
			conversion:   convert.PercentToRatio,
		},
		{
			measurements: props.QuantitativePrecipitation,
			setter:       weather.SetPreciptationAmount,
			conversion:   convert.MmToIn,
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
		{
			measurements: props.IceAccumulation,
			setter:       weather.SetIceAmount,
			conversion:   convert.MmToIn,
		},
		{
			measurements: props.SnowfallAmount,
			setter:       weather.SetSnowAmount,
			conversion:   convert.MmToIn,
		},
	}

	recordMap := make(map[time.Time]weather.Record)
	for _, items := range table {
		err := processMeasurement(&recordMap, items)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	values := make([]weather.Record, len(recordMap))
	i := 0
	for _, value := range recordMap {
		values[i] = value
		i++
	}
	return values, nil
}

func processMeasurement(recordMapP *map[time.Time]weather.Record, t transformation) error {
	recordMap := *recordMapP
	for _, forecastRecord := range t.measurements.Values {
		hours, err := durationStrToHours(forecastRecord.ValidTime)
		if err != nil {
			return errors.WithStack(err)
		}
		convertedValue := forecastRecord.Value
		if t.aggregation != nil {
			convertedValue = t.aggregation(len(hours), convertedValue)
		}
		convertedValue = t.conversion(convertedValue)
		for _, hour := range hours {
			record := recordMap[hour]
			t.setter(&record, convertedValue)
			record.Time = hour
			recordMap[hour] = record
		}
	}
	return nil
}

func durationStrToHours(dateString string) ([]time.Time, error) {
	// split string by slash
	split := strings.Split(dateString, "/")

	// calculate duration in hours
	duration, err := period.Parse(split[1])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hours := int(math.Ceil(duration.DurationApprox().Hours()))

	// parse time(hour), defaulting to UTC. for some reason Parse() doesn't work to default to UTC.
	point, err := time.ParseInLocation(time.RFC3339, split[0], nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// make a slice with all the hours contained in the duration
	times := make([]time.Time, hours)
	for i := range times {
		times[i] = point
		point = point.Add(time.Hour)
	}
	return times, nil
}

func cleanup(closer io.Closer) {
	// todo: better error handling
	if closer.Close() != nil {
		log.Fatalln("Failed to cleanup")
	}
}

type transformation struct {
	measurements forecastMeasurements
	setter       func(record *weather.Record, val float64)
	conversion   func(val float64) float64
	aggregation  func(hours int, val float64) float64
}

type forecastMeasurements struct {
	Uom    string `json:"uom"`
	Values []struct {
		ValidTime string  `json:"validTime"`
		Value     float64 `json:"value"`
	}
}

type nwsForecast struct {
	Properties struct {
		UpdateTime                 string               `json:"updateTime"`
		Temperature                forecastMeasurements `json:"temperature"`
		Dewpoint                   forecastMeasurements `json:"dewpoint"`
		ApparentTemperature        forecastMeasurements `json:"apparentTemperature"`
		SkyCover                   forecastMeasurements `json:"skyCover"`
		WindDirection              forecastMeasurements `json:"windDirection"`
		WindSpeed                  forecastMeasurements `json:"windSpeed"`
		WindGust                   forecastMeasurements `json:"windGust"`
		ProbabilityOfPrecipitation forecastMeasurements `json:"probabilityOfPrecipitation"`
		QuantitativePrecipitation  forecastMeasurements `json:"quantitativePrecipitation"`
		IceAccumulation            forecastMeasurements `json:"iceAccumulation"`
		SnowfallAmount             forecastMeasurements `json:"snowfallAmount"`
		Hazards                    struct {
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     []struct {
					Phenomenon   string      `json:"phenomenon"`
					Significance interface{} `json:"significance"`
					EventNumber  interface{} `json:"event_number"`
				} `json:"value"`
			} `json:"values"`
		} `json:"hazards"`
	} `json:"properties"`
}
