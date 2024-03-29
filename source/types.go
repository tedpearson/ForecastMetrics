package source

import (
	"time"
)

// Forecast holds a weather forecast and an astronomy forecast.
type Forecast struct {
	WeatherRecords []WeatherRecord
	AstroEvents    []AstroEvent
}

// WeatherRecord is a weather forecast for a single point in time.
type WeatherRecord struct {
	Time                     time.Time
	Temperature              *float64
	Dewpoint                 *float64
	FeelsLike                *float64
	SkyCover                 *float64
	WindDirection            *float64
	WindSpeed                *float64
	WindGust                 *float64
	PrecipitationProbability *float64
	PrecipitationAmount      *float64
	SnowAmount               *float64
	IceAmount                *float64
}

// AstroEvent is a forecast of sun and moon events.
type AstroEvent struct {
	Time   time.Time
	SunUp  *int
	MoonUp *int
	// this is hard to name. It's not "how bright is the moon" - it's "ratio of current moon phase to the full moon".
	FullMoonRatio *float64
}

// InfluxPointer is either a WeatherRecord or an AstroEvent.
type InfluxPointer interface {
	WeatherRecord | AstroEvent
}

// Forecaster can return a forecast for a given geo coordinate.
type Forecaster interface {
	GetForecast(lat string, lon string) (*Forecast, error)
}

func SetTemperature(r *WeatherRecord, v float64) {
	r.Temperature = &v
}

func SetDewpoint(r *WeatherRecord, v float64) {
	r.Dewpoint = &v
}

func SetFeelsLike(r *WeatherRecord, v float64) {
	r.FeelsLike = &v
}

func SetSkyCover(r *WeatherRecord, v float64) {
	r.SkyCover = &v
}

func SetWindDirection(r *WeatherRecord, v float64) {
	r.WindDirection = &v
}

func SetWindSpeed(r *WeatherRecord, v float64) {
	r.WindSpeed = &v
}

func SetWindGust(r *WeatherRecord, v float64) {
	r.WindGust = &v
}

func SetPrecipitationProbability(r *WeatherRecord, v float64) {
	r.PrecipitationProbability = &v
}

func SetPreciptationAmount(r *WeatherRecord, v float64) {
	r.PrecipitationAmount = &v
}

func SetSnowAmount(r *WeatherRecord, v float64) {
	r.SnowAmount = &v
}

func SetIceAmount(r *WeatherRecord, v float64) {
	r.IceAmount = &v
}
