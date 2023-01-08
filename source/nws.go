package source

// https://weather-gov.github.io/api/gridpoints
// https://weather-gov.github.io/api/general-faqs
// https://www.weather.gov/documentation/services-web-api#/default/get_gridpoints__wfo___x___y_

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"
	"github.com/rickb777/date/period"

	"github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/internal/convert"
	"github.com/tedpearson/ForecastMetrics/v3/weather"
)

type NWS struct {
	forecast nwsForecast
}

func (n *NWS) Init(lat string, lon string, retryer http.Retryer) error {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	log.Println("Looking up NWS location")

	off := backoff.NewExponentialBackOff()
	off.MaxElapsedTime = 22 * time.Second
	body1, err := retryer.RetryRequest(url, off)
	if err != nil {
		return err
	}
	defer cleanup(body1)
	var jsonResponse map[string]interface{}
	err = json.NewDecoder(body1).Decode(&jsonResponse)
	if err != nil {
		return errors.WithStack(err)
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)
	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	log.Println("Getting NWS forecast")
	body2, err := retryer.RetryRequest(gridpointUrl, off)
	if err != nil {
		return err
	}
	defer cleanup(body2)

	var forecast nwsForecast
	err = json.NewDecoder(body2).Decode(&forecast)
	if err != nil {
		return errors.WithStack(err)
	}
	n.forecast = forecast
	return nil
}

func (n *NWS) GetWeather() ([]weather.Record, error) {
	var empty []weather.Record
	records, err := n.transformForecast(n.forecast)
	if err != nil {
		return empty, err
	}
	return records, nil
}

func (n *NWS) transformForecast(forecast nwsForecast) ([]weather.Record, error) {
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
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
		{
			measurements: props.SnowfallAmount,
			setter:       weather.SetSnowAmount,
			conversion:   convert.MmToIn,
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
	}

	recordMap := make(map[time.Time]weather.Record)
	for _, items := range table {
		err := processMeasurement(&recordMap, items)
		if err != nil {
			return nil, err
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
			return err
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
	measurements nwsForecastMeasurements
	setter       func(record *weather.Record, val float64)
	conversion   func(val float64) float64
	aggregation  func(hours int, val float64) float64
}

type nwsForecastMeasurements struct {
	Uom    string `json:"uom"`
	Values []struct {
		ValidTime string  `json:"validTime"`
		Value     float64 `json:"value"`
	}
}

type nwsForecast struct {
	Properties struct {
		UpdateTime                 string                  `json:"updateTime"`
		Temperature                nwsForecastMeasurements `json:"temperature"`
		Dewpoint                   nwsForecastMeasurements `json:"dewpoint"`
		ApparentTemperature        nwsForecastMeasurements `json:"apparentTemperature"`
		SkyCover                   nwsForecastMeasurements `json:"skyCover"`
		WindDirection              nwsForecastMeasurements `json:"windDirection"`
		WindSpeed                  nwsForecastMeasurements `json:"windSpeed"`
		WindGust                   nwsForecastMeasurements `json:"windGust"`
		ProbabilityOfPrecipitation nwsForecastMeasurements `json:"probabilityOfPrecipitation"`
		QuantitativePrecipitation  nwsForecastMeasurements `json:"quantitativePrecipitation"`
		IceAccumulation            nwsForecastMeasurements `json:"iceAccumulation"`
		SnowfallAmount             nwsForecastMeasurements `json:"snowfallAmount"`
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
