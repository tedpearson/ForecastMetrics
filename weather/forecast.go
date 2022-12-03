package weather

import (
	"reflect"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/tedpearson/ForecastMetrics/http"
)

type WriteOptions struct {
	ForecastSource  string
	MeasurementName string
	Location        string
	ForecastTime    *string
}

type Record struct {
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

func RecordsToPoints(rs []Record, options WriteOptions) []*write.Point {
	events := make([]interface{}, len(rs))
	for i, event := range rs {
		events[i] = event
	}
	return toPoints(events, options)
}

type AstroEvent struct {
	Time   time.Time
	SunUp  *int
	MoonUp *int
	// this is hard to name. It's not "how bright is the moon" - it's "ratio of current moon phase to the full moon".
	FullMoonRatio *float64
}

func AstroToPoints(aes []AstroEvent, options WriteOptions) []*write.Point {
	events := make([]interface{}, len(aes))
	for i, event := range aes {
		events[i] = event
	}
	return toPoints(events, options)
}

func toPoints(items []interface{}, options WriteOptions) []*write.Point {
	points := make([]*write.Point, 0, len(items))
	for _, item := range items {
		t := reflect.ValueOf(item).FieldByName("Time").Interface().(time.Time)
		// only send future datapoints.
		ft := options.ForecastTime
		if ft != nil && *ft != "0" && t.Before(time.Now().Add(time.Hour+1)) {
			continue
		}
		points = append(points, toPoint(t, item, options))
	}
	return points
}

func toPoint(t time.Time, i interface{}, options WriteOptions) *write.Point {
	tags := map[string]string{
		"source":   options.ForecastSource,
		"location": options.Location,
	}
	if options.ForecastTime != nil {
		tags["forecast_time"] = *options.ForecastTime
	}
	fields := make(map[string]interface{})
	e := reflect.ValueOf(i)
	for i := 0; i < e.NumField(); i++ {
		name := strcase.ToSnake(e.Type().Field(i).Name)
		// note: skip time field (added when creating the point)
		if name == "time" {
			continue
		}
		ptr := e.Field(i)
		if ptr.IsNil() {
			// don't dereference nil pointers
			continue
		}
		val := ptr.Elem().Interface()
		fields[name] = val
	}
	return write.NewPoint(options.MeasurementName, tags, fields, t)
}

type Initer interface {
	Init(lat string, lon string, retryer http.Retryer) error
}

type Forecaster interface {
	Initer
	GetWeather() ([]Record, error)
}

type Astrocaster interface {
	Initer
	GetAstrocast() ([]AstroEvent, error)
}

func SetTemperature(r *Record, v float64) {
	r.Temperature = &v
}

func SetDewpoint(r *Record, v float64) {
	r.Dewpoint = &v
}

func SetFeelsLike(r *Record, v float64) {
	r.FeelsLike = &v
}

func SetSkyCover(r *Record, v float64) {
	r.SkyCover = &v
}

func SetWindDirection(r *Record, v float64) {
	r.WindDirection = &v
}

func SetWindSpeed(r *Record, v float64) {
	r.WindSpeed = &v
}

func SetWindGust(r *Record, v float64) {
	r.WindGust = &v
}

func SetPrecipitationProbability(r *Record, v float64) {
	r.PrecipitationProbability = &v
}

func SetPreciptationAmount(r *Record, v float64) {
	r.PrecipitationAmount = &v
}

func SetSnowAmount(r *Record, v float64) {
	r.SnowAmount = &v
}

func SetIceAmount(r *Record, v float64) {
	r.IceAmount = &v
}
