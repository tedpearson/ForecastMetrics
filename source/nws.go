package source

// https://weather-gov.github.io/api/gridpoints
// https://weather-gov.github.io/api/general-faqs
// https://www.weather.gov/documentation/services-web-api#/default/get_gridpoints__wfo___x___y_

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/rickb777/date/period"

	"github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/internal/convert"
)

type NWS struct {
	Retryer http.Retryer
}

func (n *NWS) GetForecast(lat string, lon string) (*Forecast, error) {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	fmt.Println("Looking up NWS location")

	off := backoff.NewExponentialBackOff()
	off.MaxElapsedTime = 22 * time.Second
	body1, err := n.Retryer.RetryRequest(url, off)
	if err != nil {
		return nil, err
	}
	defer cleanup(body1)
	var jsonResponse map[string]interface{}
	err = json.NewDecoder(body1).Decode(&jsonResponse)
	if err != nil {
		return nil, err
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)
	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	fmt.Println("Getting NWS forecast")
	body2, err := n.Retryer.RetryRequest(gridpointUrl, off)
	if err != nil {
		return nil, err
	}
	defer cleanup(body2)

	var forecast nwsForecast
	err = json.NewDecoder(body2).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	records, err := n.transformForecast(forecast)
	if err != nil {
		return nil, err
	}
	return &Forecast{
		WeatherRecords: records,
	}, nil
}

func (n *NWS) transformForecast(forecast nwsForecast) ([]WeatherRecord, error) {
	props := forecast.Properties
	var table = []transformation{
		{
			measurements: props.Temperature,
			setter:       SetTemperature,
			conversion:   convert.CToF,
		},
		{
			measurements: props.Dewpoint,
			setter:       SetDewpoint,
			conversion:   convert.CToF,
		},
		{
			measurements: props.ApparentTemperature,
			setter:       SetFeelsLike,
			conversion:   convert.CToF,
		},
		{
			measurements: props.SkyCover,
			setter:       SetSkyCover,
			conversion:   convert.PercentToRatio,
		},
		{
			measurements: props.WindDirection,
			setter:       SetWindDirection,
			conversion:   convert.Identity,
		},
		{
			measurements: props.WindSpeed,
			setter:       SetWindSpeed,
			conversion:   convert.KmhToMph,
		},
		{
			measurements: props.WindGust,
			setter:       SetWindGust,
			conversion:   convert.KmhToMph,
		},
		{
			measurements: props.ProbabilityOfPrecipitation,
			setter:       SetPrecipitationProbability,
			conversion:   convert.PercentToRatio,
		},
		{
			measurements: props.QuantitativePrecipitation,
			setter:       SetPreciptationAmount,
			conversion:   convert.MmToIn,
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
		{
			measurements: props.IceAccumulation,
			setter:       SetIceAmount,
			conversion:   convert.MmToIn,
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
		{
			measurements: props.SnowfallAmount,
			setter:       SetSnowAmount,
			conversion:   convert.MmToIn,
			aggregation: func(hours int, val float64) float64 {
				return val / float64(hours)
			},
		},
	}

	recordMap := make(map[time.Time]WeatherRecord)
	for _, items := range table {
		err := processMeasurement(&recordMap, items)
		if err != nil {
			return nil, err
		}
	}

	values := make([]WeatherRecord, len(recordMap))
	i := 0
	for _, value := range recordMap {
		values[i] = value
		i++
	}
	slices.SortFunc(values, func(a, b WeatherRecord) int {
		return a.Time.Compare(b.Time)
	})
	return values, nil
}

func processMeasurement(recordMapP *map[time.Time]WeatherRecord, t transformation) error {
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
		return nil, err
	}
	hours := int(math.Ceil(duration.DurationApprox().Hours()))

	// parse time(hour), defaulting to UTC. for some reason Parse() doesn't work to default to UTC.
	point, err := time.ParseInLocation(time.RFC3339, split[0], nil)
	if err != nil {
		return nil, err
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
		panic("Failed to cleanup")
	}
}

type transformation struct {
	measurements nwsForecastMeasurements
	setter       func(record *WeatherRecord, val float64)
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
