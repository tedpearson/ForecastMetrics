package main

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-errors/errors"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/rickb777/date/period"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// talk to nws api and get results

func GetWeather(lat string, lon string) error {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	cache := diskcache.New("/tmp/weather-cache")
	client := httpcache.NewTransport(cache).Client()
	//client := httpcache.NewMemoryCacheTransport().Client()
	body, err := makeRequest(url, client)
	if err != nil {
		return errors.New(err)
	}
	defer cleanup(body)
	var jsonResponse map[string]interface{}
	err = json.NewDecoder(body).Decode(&jsonResponse)
	if err != nil {
		return errors.New(err)
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)
	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	body, err = makeRequest(gridpointUrl, client)
	if err != nil {
		return errors.New(err)
	}
	defer cleanup(body)
	var forecast NwsForecast
	err = json.NewDecoder(body).Decode(&forecast)
	if err != nil {
		return errors.New(err)
	}
	spew.Dump(forecast)

	records, err := transformForecast(forecast)
	if err != nil {
		return errors.New(err)
	}
	spew.Dump(records)

	return nil
}

func makeRequest(url string, client *http.Client) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.New(err)
	}
	// user-agent required by weather.gov with email
	req.Header.Set("User-Agent", "weather2influxdb; by ted@tedpearson.com")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New(err)
	}
	return resp.Body, nil
}

func transformForecast(forecast NwsForecast) ([]ForecastRecord, error) {
	// use temperature as a proxy for all points referenced?
	// use a map and a slice to look up the points by time, then return them sorted
	recordMap := make(map[time.Time]ForecastRecord)

	// todo: generify similar data processing :)
	for _, tempRecord := range forecast.Properties.Temperature.Values {
		hours, err := DurationStrToHours(tempRecord.ValidTime)
		if err != nil {
			return nil, errors.New(err)
		}
		for _, hour := range hours {
			record := recordMap[hour]
			record.Temperature = tempRecord.Value
			record.Time = hour
			recordMap[hour] = record
		}
	}
	spew.Dump(recordMap)

	// get slice of keys
	keys := make([]time.Time, len(recordMap))
	i := 0
	for key := range recordMap {
		keys[i] = key
		i++
	}
	// sort keys, and return values in that order
	// note: sorting the output may not actually be important
	//  what is important is that the points are written correctly :)
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	values := make([]ForecastRecord, len(keys))
	for i, key := range keys {
		values[i] = recordMap[key]
	}
	return values, nil
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

type NwsForecast struct {
	Properties struct {
		UpdateTime  string `json:"updateTime"`
		Temperature struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"temperature"`
		Dewpoint struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"dewpoint"`
		ApparentTemperature struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"apparentTemperature"`
		SkyCover struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     int    `json:"value"`
			} `json:"values"`
		} `json:"skyCover"`
		WindDirection struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     int    `json:"value"`
			} `json:"values"`
		} `json:"windDirection"`
		WindSpeed struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"windSpeed"`
		WindGust struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"windGust"`
		Hazards struct {
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     []struct {
					Phenomenon   string      `json:"phenomenon"`
					Significance interface{} `json:"significance"`
					EventNumber  interface{} `json:"event_number"`
				} `json:"value"`
			} `json:"values"`
		} `json:"hazards"`
		ProbabilityOfPrecipitation struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     int    `json:"value"`
			} `json:"values"`
		} `json:"probabilityOfPrecipitation"`
		QuantitativePrecipitation struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string  `json:"validTime"`
				Value     float64 `json:"value"`
			} `json:"values"`
		} `json:"quantitativePrecipitation"`
		IceAccumulation struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     int    `json:"value"`
			} `json:"values"`
		} `json:"iceAccumulation"`
		SnowfallAmount struct {
			Uom    string `json:"uom"`
			Values []struct {
				ValidTime string `json:"validTime"`
				Value     int    `json:"value"`
			} `json:"values"`
		} `json:"snowfallAmount"`
	} `json:"properties"`
}
