package http

import (
	"fmt"
	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
)

func RetryRequest(url string, client *http.Client, off *backoff.ExponentialBackOff) (io.ReadCloser, error) {
	var body *io.ReadCloser
	err := backoff.Retry(doRequest(url, client, &body), off)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return *body, nil
}

func doRequest(url string, client *http.Client, body **io.ReadCloser) func() error {
	return func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return backoff.Permanent(err)
		}
		// user-agent required by weather.gov with email
		req.Header.Set("User-Agent", "https://github.com/tedpearson/weather2influxdb by ted@tedpearson.com")
		resp, err := client.Do(req)
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