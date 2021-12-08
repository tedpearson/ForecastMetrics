package source

import (
	"encoding/json"
	"log"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"
	"github.com/tedpearson/weather2influxdb/convert"
	"github.com/tedpearson/weather2influxdb/http"
	"github.com/tedpearson/weather2influxdb/weather"
)

type TheGlobalWeather struct {
	Key      string
	forecast tgwForecast
}

func (t *TheGlobalWeather) Init(lat string, lon string, retryer http.Retryer) error {
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
		return err
	}
	defer cleanup(body)

	var forecast tgwForecast
	err = json.NewDecoder(body).Decode(&forecast)
	if err != nil {
		return errors.WithStack(err)
	}
	t.forecast = forecast
	return nil
}

func (t *TheGlobalWeather) GetWeather() (weather.Records, error) {
	empty := weather.Records{}
	records := make([]weather.Record, 0, (24*10)+(5*16))
	loc, err := time.LoadLocation(t.forecast.Location.Tzid)
	if err != nil {
		return empty, errors.WithStack(err)
	}
	for _, day := range t.forecast.Forecast.Forecastday {
		for _, hour := range day.Hour {
			record, err := t.processHour(hour, loc)
			if err != nil {
				return empty, err
			}
			records = append(records, record)
		}
	}
	return weather.Records{Values: records}, nil
}

func (t *TheGlobalWeather) GetAstrocast() (weather.AstroEvents, error) {
	events := make([]weather.AstroEvent, 0, 5*16)
	loc, err := time.LoadLocation(t.forecast.Location.Tzid)
	if err != nil {
		return weather.AstroEvents{}, errors.WithStack(err)
	}
	for _, day := range t.forecast.Forecast.Forecastday {
		a := day.Astro
		one := 1
		zero := 0
		t.processAstro(astroConversion{
			time:   a.Sunrise,
			date:   day.Date,
			loc:    loc,
			f:      func(r *weather.AstroEvent) { r.SunUp = &one },
			events: &events,
		})
		t.processAstro(astroConversion{
			time:   a.Sunset,
			date:   day.Date,
			loc:    loc,
			f:      func(r *weather.AstroEvent) { r.SunUp = &zero },
			events: &events,
		})
		t.processAstro(astroConversion{
			time:   a.Moonrise,
			date:   day.Date,
			loc:    loc,
			f:      func(r *weather.AstroEvent) { r.MoonUp = &one },
			events: &events,
		})
		t.processAstro(astroConversion{
			time:   a.Moonset,
			date:   day.Date,
			loc:    loc,
			f:      func(r *weather.AstroEvent) { r.MoonUp = &zero },
			events: &events,
		})
		t.processAstro(astroConversion{
			time: a.Sunset,
			date: day.Date,
			loc:  loc,
			f: func(r *weather.AstroEvent) {
				f := convert.StrToF(a.MoonIllumination, "moon illumination")
				ratio := convert.PercentToRatio(f)
				r.FullMoonRatio = &ratio
			},
			events: &events,
		})
	}
	return weather.AstroEvents{Values: events}, nil
}

func (t *TheGlobalWeather) processHour(hour Hour, loc *time.Location) (weather.Record, error) {
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
	}
	return record, nil
}

type astroConversion struct {
	time   string
	date   string
	loc    *time.Location
	f      func(record *weather.AstroEvent)
	events *[]weather.AstroEvent
}

func (t *TheGlobalWeather) processAstro(c astroConversion) {
	joined := c.date + " " + c.time
	stamp, err := time.ParseInLocation("2006-01-02 03:04 PM", joined, c.loc)
	if err != nil {
		// if there are issues with astro, this is a good place to debug
		// note: we don't do anything here because moonrise/moonset fields sometiimes say "no xxx" as the value
		//  and that's okay, we don't want to do anything with the value.
		return
	}
	r := weather.AstroEvent{
		Time: stamp,
	}
	c.f(&r)
	*c.events = append(*c.events, r)
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
