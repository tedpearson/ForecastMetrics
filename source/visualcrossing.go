package source

import (
	"encoding/json"
	"math"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v3"

	"github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/internal/convert"
)

// VisualCrossing provides weather forecasts from https://www.visualcrossing.com
// VisualCrossing supports astronomy forecasts.
type VisualCrossing struct {
	Retryer http.Retryer
	Key     string
}

// GetForecast implements Forecaster by returning the VisualCrossing weather and astronomy forecasts.
func (v *VisualCrossing) GetForecast(lat string, lon string) (*Forecast, error) {
	base := "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/weatherdata/forecast?"
	q := url.Values{}
	q.Add("aggregateHours", "1")
	q.Add("contentType", "json")
	q.Add("unitGroup", "us")
	q.Add("locationMode", "single")
	q.Add("key", v.Key)
	q.Add("location", lat+","+lon)
	q.Add("includeAstronomy", "true")
	off := backoff.NewExponentialBackOff()
	// note: low number of retries because we are using free tier (250 results/day)
	off.MaxElapsedTime = 4 * time.Second
	body, err := v.Retryer.RetryRequest(base+q.Encode(), off)
	if err != nil {
		return nil, err
	}
	defer cleanup(body)

	var forecast vcForecast
	err = json.NewDecoder(body).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	weatherRecords := make([]WeatherRecord, 0, len(forecast.Location.Values))
	for _, m := range forecast.Location.Values {
		// note: after 7 days, the forecast data is every 3 hours
		//       but the other 2 hours are still in the output
		//       with null values for everything except precip/datetime/datetimeStr
		//       therefore it is important to skip these, which all have null temps.
		if m.Temp == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, m.DatetimeStr)
		if err != nil {
			return nil, err
		}
		skyCover := convert.PercentToRatio(*m.CloudCover)
		var precipProb *float64
		if m.Pop != nil {
			pop := convert.PercentToRatio(*m.Pop)
			precipProb = &pop
		}
		record := WeatherRecord{
			Time:                     t,
			Temperature:              m.Temp,
			Dewpoint:                 calcDewpoint(*m.Humidity, *m.Temp),
			FeelsLike:                feelsLike(m.Temp, m.HeatIndex, m.WindChill),
			SkyCover:                 &skyCover,
			WindDirection:            m.Wdir,
			WindSpeed:                m.Wspd,
			WindGust:                 m.Wgust,
			PrecipitationProbability: precipProb,
			PrecipitationAmount:      m.Precip,
			SnowAmount:               convert.NilToZero(m.Snow),
		}
		weatherRecords = append(weatherRecords, record)
	}

	// add 32 points for sunrise and sunset each day
	astroEvents := make([]AstroEvent, 0, len(forecast.Location.Values)+32)
	one := 1
	zero := 0
	for _, m := range forecast.Location.Values {
		if m.Temp == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, m.DatetimeStr)
		if err != nil {
			return nil, err
		}
		sunrise, err := time.Parse(time.RFC3339, *m.Sunrise)
		if err != nil {
			return nil, err
		}
		sunset, err := time.Parse(time.RFC3339, *m.Sunset)
		if err != nil {
			return nil, err
		}
		// if hour < sunrise or > sunset, 0.
		// else 1.
		sunUp := &one
		if t.Before(sunrise) || t.After(sunset) {
			sunUp = &zero
		}
		astroEvents = append(astroEvents, AstroEvent{
			Time:  t,
			SunUp: sunUp,
		})
		// if this is the hour before sunrise, insert sunrise
		if sunrise.Truncate(time.Hour).Equal(t) {
			astroEvents = append(astroEvents, AstroEvent{
				Time:  sunrise,
				SunUp: &one,
			})
		}
		// if this is the hour before sunset, insert sunset + moon ratio
		if sunset.Truncate(time.Hour).Equal(t) {
			// visualcrossing moon phase:
			// 0   = new moon
			// 0.5 = full moon
			// 1   = new moon again
			moonRatio := 1 - convert.Round(2.0*math.Abs(*m.MoonPhase-0.5), 2)
			astroEvents = append(astroEvents, AstroEvent{
				Time:          sunset,
				SunUp:         &zero,
				FullMoonRatio: &moonRatio,
			})
		}
	}

	return &Forecast{
		WeatherRecords: weatherRecords,
		AstroEvents:    astroEvents,
	}, nil
}

// feelsLike combines temp, heatIndex, and windChill into a single value.
func feelsLike(temp *float64, heatIndex *float64, windChill *float64) *float64 {
	if windChill != nil {
		return windChill
	}
	if heatIndex != nil {
		return heatIndex
	}
	return temp
}

// calcDewpoint calculates dewpoint given the relative humidity and the temperature in Fahrenheit.
func calcDewpoint(rh float64, tempF float64) *float64 {
	tempC := convert.FToC(tempF)
	dpC := (237.3 * (math.Log(rh/100) + ((17.27 * tempC) / (237.3 + tempC)))) /
		(17.27 - (math.Log(rh/100) + ((17.27 * tempC) / (237.3 + tempC))))
	f := convert.CToF(dpC)
	return &f
}

// vcMeasurement is the json representation of a forecast point from VisualCrossing
type vcMeasurement struct {
	Wdir        *float64 `json:"wdir"`
	Temp        *float64 `json:"temp"`
	Sunrise     *string  `json:"sunrise"`
	Wspd        *float64 `json:"wspd"`
	DatetimeStr string   `json:"datetimeStr"`
	HeatIndex   *float64 `json:"heatindex"`
	Humidity    *float64 `json:"humidity"`
	CloudCover  *float64 `json:"cloudcover"`
	Pop         *float64 `json:"pop"`
	Datetime    int64    `json:"datetime"`
	Precip      *float64 `json:"precip"`
	Snow        *float64 `json:"snow"`
	Sunset      *string  `json:"sunset"`
	Wgust       *float64 `json:"wgust"`
	WindChill   *float64 `json:"windchill"`
	MoonPhase   *float64 `json:"moonphase"`
}

// vcForecast is the json representation of a forecast from VisualCrossing
type vcForecast struct {
	Location struct {
		Values []vcMeasurement `json:"values"`
	} `json:"location"`
}
