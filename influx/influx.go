package influx

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/iancoleman/strcase"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/tedpearson/weather2influxdb/weather"
	"log"
	"reflect"
	"time"
)

type Writer struct {
	client influxdb2.Client
}

type Config struct {
	Host     string
	User     string
	Password string
	Database string
}

type WriteOptions struct {
	Bucket          string
	ForecastSource  string
	MeasurementName string
	Location        string
	ForecastTime    int64
}

func New(config Config) Writer {
	return Writer{influxdb2.NewClient(config.Host, config.User+":"+config.Password)}
}

func (w *Writer) WriteMeasurements(options WriteOptions, measurements []weather.Record) error {
	log.Printf(`Writing %d points to "%s" in InfluxDB for "%s"`, len(measurements), options.MeasurementName,
		options.ForecastSource)
	writeApi := w.client.WriteAPIBlocking("", options.Bucket)
	for _, measurement := range measurements {
		point := makePoint(measurement, options)
		err := writeApi.WritePoint(context.Background(), point)
		if err != nil {
			return errors.New(err)
		}
	}
	return nil
}

func makePoint(record weather.Record, options WriteOptions) *write.Point {
	e := reflect.ValueOf(record)
	p := influxdb2.NewPointWithMeasurement(options.MeasurementName).
		AddTag("source", options.ForecastSource).
		AddTag("location", options.Location).
		SetTime(record.Time).
		AddField("forecast_time", options.ForecastTime)
	time.Now().UnixNano()
	for i := 0; i < e.NumField(); i++ {
		name := strcase.ToSnake(e.Type().Field(i).Name)
		if name == "Time" {
			continue
		}
		val := e.Field(i).Interface()
		p.AddField(name, val)
	}
	return p
}
