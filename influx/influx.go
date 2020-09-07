package influx

import (
	"context"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/pkg/errors"
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

func New(config Config) Writer {
	return Writer{influxdb2.NewClient(config.Host, config.User+":"+config.Password)}
}

func (w *Writer) WriteMeasurements(bucket string, points []*write.Point) error {
	writeApi := w.client.WriteAPIBlocking("", bucket)
	for _, point := range points {
		err := writeApi.WritePoint(context.Background(), point)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}
