package weather

import "time"

type ForecastRecord struct {
	Time                     time.Time
	Temperature              float64
	Dewpoint                 float64
	FeelsLike                float64
	SkyCover                 float64
	WindDirection            int
	WindSpeed                float64
	WindGust                 float64
	PrecipitationProbability float64
	PrecipitationAmount      float64
	SnowAmount               float64
	IceAmount                float64
}

type Forecaster interface {
	GetWeather(string, string, string) ([]ForecastRecord, error)
}