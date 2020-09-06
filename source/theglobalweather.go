package source

import (
	"encoding/json"
	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"
	"github.com/tedpearson/weather2influxdb/convert"
	"github.com/tedpearson/weather2influxdb/http"
	"github.com/tedpearson/weather2influxdb/weather"
	"log"
	"net/url"
	"time"
)

type TheGlobalWeather struct {
	Key string
}

func (t TheGlobalWeather) GetWeather(lat string, lon string, retryer http.Retryer) ([]weather.Record, error) {
	base := "http://api.theglobalweather.com/v1/forecast.json?"
	q := url.Values{}
	q.Add("key", t.Key)
	q.Add("q", lat+","+lon)
	q.Add("days", "10")

	off := backoff.NewExponentialBackOff()
	off.MaxElapsedTime = 10 * time.Second
	log.Println("Getting TheGlobalWeather forecast")
	body, err := retryer.RetryRequest(base+q.Encode(), off)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer cleanup(body)

	var forecast tgwForecast
	err = json.NewDecoder(body).Decode(&forecast)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	records, err := t.transformForecast(forecast)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return records, nil
}

func (t TheGlobalWeather) transformForecast(forecast tgwForecast) ([]weather.Record, error) {
	records := make([]weather.Record, 0, (24*10)+(5*16))
	loc, err := time.LoadLocation(forecast.Location.Tzid)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, day := range forecast.Forecast.Forecastday {
		t.appendAstroRecords(day, loc, &records)
		for _, hour := range day.Hour {
			record, err := t.processHour(hour, loc)
			if err != nil {
				return nil, err
			}
			records = append(records, record)
		}
	}
	return records, nil
}

func (t TheGlobalWeather) processHour(hour Hour, loc *time.Location) (weather.Record, error) {
	stamp, err := time.ParseInLocation("2006-01-02 15:04", hour.Time, loc)
	if err != nil {
		return weather.Record{}, errors.WithStack(err)
	}
	skyCover := convert.PercentToRatio(float64(hour.Cloud))
	precipProb := convert.PercentToRatio(convert.StrToF(hour.ChanceOfRain, "chance of rain"))
	record := weather.Record{
		Time:                     stamp,
		Temperature:              &hour.TempF,
		Dewpoint:                 &hour.DewpointF,
		FeelsLike:                &hour.FeelslikeF,
		SkyCover:                 &skyCover,
		WindDirection:            &hour.WindDegree,
		WindSpeed:                &hour.WindMph,
		WindGust:                 &hour.GustMph,
		PrecipitationProbability: &precipProb,
		PrecipitationAmount:      &hour.PrecipIn,
		SnowAmount:               nil,
		IceAmount:                nil,
	}
	return record, nil
}

func (t TheGlobalWeather) appendAstroRecords(day Forecastday, loc *time.Location, records *[]weather.Record) {
	a := day.Astro
	one := 1
	zero := 0
	t.processAstro(astroConversion{
		time:    a.Sunrise,
		date:    day.Date,
		loc:     loc,
		f:       func(r *weather.Record) { r.SunUp = &one },
		records: records,
	})
	t.processAstro(astroConversion{
		time:    a.Sunset,
		date:    day.Date,
		loc:     loc,
		f:       func(r *weather.Record) { r.SunUp = &zero },
		records: records,
	})
	t.processAstro(astroConversion{
		time:    a.Moonrise,
		date:    day.Date,
		loc:     loc,
		f:       func(r *weather.Record) { r.MoonUp = &one },
		records: records,
	})
	t.processAstro(astroConversion{
		time:    a.Moonset,
		date:    day.Date,
		loc:     loc,
		f:       func(r *weather.Record) { r.MoonUp = &zero },
		records: records,
	})
	t.processAstro(astroConversion{
		time: a.Sunset,
		date: day.Date,
		loc:  loc,
		f: func(r *weather.Record) {
			f := convert.StrToF(a.MoonIllumination, "moon illumination")
			ratio := convert.PercentToRatio(f)
			r.MoonPhase = &ratio
		},
		records: records,
	})
}

type astroConversion struct {
	time    string
	date    string
	loc     *time.Location
	f       func(record *weather.Record)
	records *[]weather.Record
}

func (t TheGlobalWeather) processAstro(c astroConversion) {
	joined := c.date + " " + c.time
	stamp, err := time.ParseInLocation("2006-01-02 03:04 PM", joined, c.loc)
	if err != nil {
		// if there are issues with astro, this is a good place to debug
		// note: we don't do anything here because moonrise/moonset fields sometiimes say "no xxx" as the value
		//  and that's okay, we don't want to do anything with the value.
		return
	}
	r := weather.Record{
		Time: stamp,
	}
	c.f(&r)
	*c.records = append(*c.records, r)
}

type tgwForecast struct {
	Forecast Forecast `json:"forecast"`
	Location struct {
		Tzid string `json:"tz_id"`
	} `json:"location"`
}

type Astro struct {
	Sunrise          string `json:"sunrise"`
	Sunset           string `json:"sunset"`
	Moonrise         string `json:"moonrise"`
	Moonset          string `json:"moonset"`
	MoonIllumination string `json:"moon_illumination"`
}

type Hour struct {
	TimeEpoch    int     `json:"time_epoch"`
	Time         string  `json:"time"`
	TempC        float64 `json:"temp_c"`
	TempF        float64 `json:"temp_f"`
	WindMph      float64 `json:"wind_mph"`
	WindKph      float64 `json:"wind_kph"`
	WindDegree   int     `json:"wind_degree"`
	PressureMb   float64 `json:"pressure_mb"`
	PressureIn   float64 `json:"pressure_in"`
	PrecipMm     float64 `json:"precip_mm"`
	PrecipIn     float64 `json:"precip_in"`
	Cloud        int     `json:"cloud"`
	FeelslikeC   float64 `json:"feelslike_c"`
	FeelslikeF   float64 `json:"feelslike_f"`
	DewpointC    float64 `json:"dewpoint_c"`
	DewpointF    float64 `json:"dewpoint_f"`
	ChanceOfRain string  `json:"chance_of_rain"`
	ChanceOfSnow string  `json:"chance_of_snow"`
	GustMph      float64 `json:"gust_mph"`
	GustKph      float64 `json:"gust_kph"`
}

type Forecastday struct {
	Date      string `json:"date"`
	DateEpoch int    `json:"date_epoch"`
	Astro     Astro  `json:"astro"`
	Hour      []Hour `json:"hour"`
}

type Forecast struct {
	Forecastday []Forecastday `json:"forecastday"`
}
