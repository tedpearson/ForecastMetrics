package influx

import (
	influxdb1 "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
)

type Writer struct {
	client influxdb1.Client
}

type Config struct {
	Host     string
	User     string
	Password string
	Database string
}

func New(config Config) (*Writer, error) {
	client, err := influxdb1.NewHTTPClient(influxdb1.HTTPConfig{
		Addr:     config.Host,
		Username: config.User,
		Password: config.Password,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Writer{client}, nil
}

func (w *Writer) WriteMeasurements(points influxdb1.BatchPoints, err error) error {
	if err != nil {
		return err
	}
	err = w.client.Write(points)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
