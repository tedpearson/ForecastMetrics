package weather

import (
	"github.com/tedpearson/ForecastMetrics/v3/http"
	"github.com/tedpearson/ForecastMetrics/v3/source"
)

type WriteOptions struct {
	ForecastSource  string
	MeasurementName string
	Location        string
	ForecastTime    *string
}

// Deprecated: gonna be getting rid of this in favor of ForecasterV2
type Initer interface {
	Init(lat string, lon string, retryer http.Retryer) error
}

// Deprecated: gonna be getting rid of this in favor of ForecasterV2
type Forecaster interface {
	Initer
	GetWeather() ([]source.WeatherRecord, error)
}

// Deprecated: gonna be getting rid of this in favor of ForecasterV2
type Astrocaster interface {
	Initer
	GetAstrocast() ([]source.AstroEvent, error)
}
