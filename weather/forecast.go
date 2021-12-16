package weather

import (
	"log"
	"reflect"
	"time"

	"github.com/iancoleman/strcase"
	influxdb1 "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/tedpearson/weather2influxdb/http"
)

type WriteOptions struct {
	Database        string
	ForecastSource  string
	MeasurementName string
	Location        string
	ForecastTime    string
}

type Record struct {
	Time                     time.Time
	Temperature              *float64
	Dewpoint                 *float64
	FeelsLike                *float64
	SkyCover                 *float64
	WindDirection            *int
	WindSpeed                *float64
	WindGust                 *float64
	PrecipitationProbability *float64
	PrecipitationAmount      *float64
	SnowAmount               *float64
	IceAmount                *float64
}

type Records struct {
	Values []Record
}

func (rs Records) ToPoints(options WriteOptions) (influxdb1.BatchPoints, error) {
	events := make([]interface{}, len(rs.Values))
	for i, event := range rs.Values {
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

type AstroEvents struct {
	Values []AstroEvent
}

func (as AstroEvents) ToPoints(options WriteOptions) (influxdb1.BatchPoints, error) {
	events := make([]interface{}, len(as.Values))
	for i, event := range as.Values {
		events[i] = event
	}
	return toPoints(events, options)
}

func toPoints(items []interface{}, options WriteOptions) (influxdb1.BatchPoints, error) {
	batchPoints, err := influxdb1.NewBatchPoints(influxdb1.BatchPointsConfig{
		Database: options.Database,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, item := range items {
		t := reflect.ValueOf(item).FieldByName("Time").Interface().(time.Time)
		// only send future datapoints.
		if options.ForecastTime != "0" && t.Before(time.Now().Add(time.Hour+1)) {
			continue
		}
		point, err := toPoint(t, item, options)
		if err != nil {
			log.Printf("Failed to create point: %+v", err)
			continue
		}
		batchPoints.AddPoint(point)
	}
	return batchPoints, nil
}

func toPoint(t time.Time, i interface{}, options WriteOptions) (*influxdb1.Point, error) {
	tags := map[string]string{
		"source":        options.ForecastSource,
		"location":      options.Location,
		"forecast_time": options.ForecastTime,
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
	return influxdb1.NewPoint(options.MeasurementName, tags, fields, t)
}

type Initer interface {
	Init(lat string, lon string, retryer http.Retryer) error
}

type Forecaster interface {
	Initer
	GetWeather() (Records, error)
}

type Astrocaster interface {
	Initer
	GetAstrocast() (AstroEvents, error)
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
	i := int(v)
	r.WindDirection = &i
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
