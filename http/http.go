package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cenkalti/backoff/v3"
)

type Retryer struct {
	Client *http.Client
}

func (r Retryer) RetryRequest(url string, off *backoff.ExponentialBackOff) (io.ReadCloser, error) {
	var body *io.ReadCloser
	err := backoff.Retry(r.doRequest(url, &body), off)
	if err != nil {
		return nil, err
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
		req.Header.Set("User-Agent", "https://github.com/tedpearson/ForecastMetrics by ted@tedpearson.com")
		resp, err := r.Client.Do(req)
		if err != nil {
			return backoff.Permanent(err)
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return backoff.Permanent(fmt.Errorf("http error for url %s: %s", url, resp.Status))
		}
		if resp.StatusCode != 200 {
			msg := fmt.Sprintf("Error status %d: %s", resp.StatusCode, resp.Status)
			fmt.Println(msg)
			return errors.New(msg)
		}
		*body = &resp.Body
		return nil
	}
}
