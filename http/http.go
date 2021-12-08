package http

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"
)

type Retryer struct {
	Client *http.Client
}

func (r Retryer) RetryRequest(url string, off *backoff.ExponentialBackOff) (io.ReadCloser, error) {
	var body *io.ReadCloser
	err := backoff.Retry(r.doRequest(url, &body), off)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return *body, nil
}

func (r Retryer) doRequest(url string, body **io.ReadCloser) func() error {
	return func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return backoff.Permanent(err)
		}
		// user-agent required by weather.gov with email
		req.Header.Set("User-Agent", "https://github.com/tedpearson/weather2influxdb by ted@tedpearson.com")
		resp, err := r.Client.Do(req)
		if err != nil {
			return backoff.Permanent(err)
		}
		if resp.StatusCode != 200 {
			msg := fmt.Sprintf("Error status %d: %s", resp.StatusCode, resp.Status)
			log.Println(msg)
			return errors.New(msg)
		}
		*body = &resp.Body
		return nil
	}
}
