package weather

import "time"

type Record struct {
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
	GetWeather(lat, string, lon string, cachePath string) ([]Record, error)
}

func SetTemperature(r *Record, v float64) {
	r.Temperature = v
}

func SetDewpoint(r *Record, v float64) {
	r.Dewpoint = v
}

func SetFeelsLike(r *Record, v float64) {
	r.FeelsLike = v
}

func SetSkyCover(r *Record, v float64) {
	r.SkyCover = v
}

func SetWindDirection(r *Record, v float64) {
	r.WindDirection = int(v)
}

func SetWindSpeed(r *Record, v float64) {
	r.WindSpeed = v
}

func SetWindGust(r *Record, v float64) {
	r.WindGust = v
}

func SetPrecipitationProbability(r *Record, v float64) {
	r.PrecipitationProbability = v
}

func SetPreciptationAmount(r *Record, v float64) {
	r.PrecipitationAmount = v
}

func SetSnowAmount(r *Record, v float64) {
	r.SnowAmount = v
}

func SetIceAmount(r *Record, v float64) {
	r.IceAmount = v
}
