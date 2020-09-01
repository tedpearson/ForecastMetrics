package nws

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/rickb777/date/period"
	"github.com/tedpearson/weather2influxdb/convert"
	"github.com/tedpearson/weather2influxdb/weather"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

type NWS struct{}

// talk to nws api and get results

func (n NWS) GetWeather(lat string, lon string, cachePath string) ([]weather.Record, error) {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	cache := diskcache.New(cachePath)
	client := httpcache.NewTransport(cache).Client()
	//client := httpcache.NewMemoryCacheTransport().Client()
	log.Println("Looking up NWS location")
	body, err := makeRequest(url, client)
	if err != nil {
		return nil, errors.New(err)
	}
	defer cleanup(body)
	var jsonResponse map[string]interface{}
	err = json.NewDecoder(body).Decode(&jsonResponse)
	if err != nil {
		return nil, errors.New(err)
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)
	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	log.Println("Getting NWS forecast")
	body, err = makeRequest(gridpointUrl, client)
	if err != nil {
		return nil, errors.New(err)
	}
	defer cleanup(body)
	var forecast NwsForecast
	err = json.NewDecoder(body).Decode(&forecast)
	if err != nil {
		return nil, errors.New(err)
	}

	records, err := transformForecast(forecast)
	if err != nil {
		return nil, errors.New(err)
	}
	return records, nil
}

func makeRequest(url string, client *http.Client) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.New(err)
	}
	// user-agent required by weather.gov with email
	req.Header.Set("User-Agent", "https://github.com/tedpearson/weather2influxdb by ted@tedpearson.com")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New(err)
	}
	return resp.Body, nil
}

func transformForecast(forecast NwsForecast) ([]weather.Record, error) {
	props := forecast.Properties
	var table = []Transformation{
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
			return nil, errors.New(err)
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

func processMeasurement(recordMapP *map[time.Time]weather.Record, t Transformation) error {
	recordMap := *recordMapP
	for _, forecastRecord := range t.measurements.Values {
		hours, err := DurationStrToHours(forecastRecord.ValidTime)
		if err != nil {
			return errors.New(err)
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

func DurationStrToHours(dateString string) ([]time.Time, error) {
	// split string by slash
	split := strings.Split(dateString, "/")

	// calculate duration in hours
	duration, err := period.Parse(split[1])
	if err != nil {
		return nil, errors.New(err)
	}
	hours := int(math.Ceil(duration.DurationApprox().Hours()))

	// parse time(hour), defaulting to UTC. for some reason Parse() doesn't work to default to UTC.
	point, err := time.ParseInLocation(time.RFC3339, split[0], nil)
	if err != nil {
		return nil, errors.New(err)
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

type Transformation struct {
	measurements ForecastMeasurements
	setter       func(record *weather.Record, val float64)
	conversion   func(val float64) float64
	aggregation  func(hours int, val float64) float64
}

type ForecastMeasurements struct {
	Uom    string `json:"uom"`
	Values []struct {
		ValidTime string  `json:"validTime"`
		Value     float64 `json:"value"`
	}
}

type NwsForecast struct {
	Properties struct {
		UpdateTime                 string               `json:"updateTime"`
		Temperature                ForecastMeasurements `json:"temperature"`
		Dewpoint                   ForecastMeasurements `json:"dewpoint"`
		ApparentTemperature        ForecastMeasurements `json:"apparentTemperature"`
		SkyCover                   ForecastMeasurements `json:"skyCover"`
		WindDirection              ForecastMeasurements `json:"windDirection"`
		WindSpeed                  ForecastMeasurements `json:"windSpeed"`
		WindGust                   ForecastMeasurements `json:"windGust"`
		ProbabilityOfPrecipitation ForecastMeasurements `json:"probabilityOfPrecipitation"`
		QuantitativePrecipitation  ForecastMeasurements `json:"quantitativePrecipitation"`
		IceAccumulation            ForecastMeasurements `json:"iceAccumulation"`
		SnowfallAmount             ForecastMeasurements `json:"snowfallAmount"`
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
