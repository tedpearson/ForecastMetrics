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
	"reflect"
	"strings"
	"time"
)

type NWS struct {}

// talk to nws api and get results

func (n NWS) GetWeather(lat string, lon string, cachePath string) ([]weather.ForecastRecord, error) {
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

func transformForecast(forecast NwsForecast) ([]weather.ForecastRecord, error) {
	var table = []Transformation{
		{forecast.Properties.Temperature, "Temperature", convert.CToF},
		{forecast.Properties.Dewpoint, "Dewpoint", convert.CToF},
		{forecast.Properties.ApparentTemperature, "FeelsLike", convert.CToF},
		{forecast.Properties.SkyCover, "SkyCover", convert.PercentToRatio},
		{forecast.Properties.WindDirection, "WindDirection", convert.Identity},
		{forecast.Properties.WindSpeed, "WindSpeed", convert.KmhToMph},
		{forecast.Properties.WindGust, "WindGust", convert.KmhToMph},
		{forecast.Properties.ProbabilityOfPrecipitation, "PrecipitationProbability", convert.PercentToRatio},
		{forecast.Properties.QuantitativePrecipitation, "PrecipitationAmount", convert.MmToIn},
		{forecast.Properties.IceAccumulation, "IceAmount", convert.MmToIn},
		{forecast.Properties.SnowfallAmount, "SnowAmount", convert.MmToIn},
	}

	recordMap := make(map[time.Time]weather.ForecastRecord)
	for _, items := range table {
		err := processMeasurement(&recordMap, items)
		if err != nil {
			return nil, errors.New(err)
		}
	}

	values := make([]weather.ForecastRecord, len(recordMap))
	i := 0
	for _, value := range recordMap {
		values[i] = value
		i++
	}
	return values, nil
}

func processMeasurement(recordMapP *map[time.Time]weather.ForecastRecord, t Transformation) error {
	recordMap := *recordMapP
	for _, forecastRecord := range t.measurements.Values {
		hours, err := DurationStrToHours(forecastRecord.ValidTime)
		if err != nil {
			return errors.New(err)
		}
		if t.genericName == "PrecipitationAmount" {
			forecastRecord.Value = forecastRecord.Value / float64(len(hours))
		}
		convertedValue := t.conversion(forecastRecord.Value)
		for _, hour := range hours {
			record := recordMap[hour]
			field := reflect.ValueOf(&record).Elem().FieldByName(t.genericName)
			f := field.Kind()
			switch f {
			case reflect.Float64:
				field.SetFloat(convertedValue)
			case reflect.Int:
				field.SetInt(int64(convertedValue))
			default:
				panic(fmt.Sprintf("Unsupported ForecastRecord type: %v", f))
			}
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
	genericName  string
	conversion   func(float64) float64
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
