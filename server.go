package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Server provides the promethus endpoint for ForecastMetrics.
type Server struct {
	LocationService LocationService
	Dispatcher      *Dispatcher
	ConfigService   ConfigService
	AuthToken       string
}

// Start starts the prometheus endpoint.
func (s *Server) Start(port int64) {
	http.Handle("/", s)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		panic(err)
	}
}

// ServeHTTP implements http.Handler by serving prometheus metrics for specially formed
// prometheus http requests. If a parsed location is already written to the database,
// we proxy the prometheus request to the database.
func (s *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// handle auth
	if !Auth(req.Header.Get("Authorization"), s.AuthToken) {
		resp.Header().Set("WWW-Authenticate", `Basic realm="ForecastMetrics", charset="UTF-8"`)
		resp.WriteHeader(http.StatusUnauthorized)
		return
	}
	// get params
	body, err := io.ReadAll(req.Body)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	// prepare body to be read by parseparams
	req.Body = io.NopCloser(bytes.NewReader(body))
	params, err := s.ParseParams(req)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Printf("Failed to parse params: %s\n", err)
		return
	}

	// check if we should proxy
	proxying := ""
	if s.ConfigService.HasLocation(params.Location) {
		// prepare body to be read by reverse proxy
		req.Body = io.NopCloser(bytes.NewReader(body))
		Proxy(resp, req, *params)
		proxying = " (proxying)"
		return
	}
	fmt.Printf("Request%s: %s\n", proxying, params)

	forecast, err := s.Dispatcher.GetForecast(params.Location, params.Source, params.AdHoc)
	if err != nil {
		fmt.Printf("Error getting forecast: %+v\n", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	// convert to prometheus response.
	promResponse := ConvertToTimeSeries(*forecast, *params)
	// send prom response as json to client
	resp.Header().Add("content-type", "application/json")
	respJson, err := json.Marshal(promResponse)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = resp.Write(respJson)
	if err != nil {
		fmt.Printf("Error writing response to client: %+v\n", err)
	}
}

// Auth checks if the authHeader passes Basic authentication with the configured credentials.
func Auth(authHeader, authToken string) bool {
	if token, ok := strings.CutPrefix(authHeader, "Basic "); ok {
		b, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return false
		}
		return string(b) == authToken
	}
	return false
}

// Proxy proxies the request to the database.
func Proxy(resp http.ResponseWriter, req *http.Request, params Params) {
	// fixme: configure url
	u, _ := url.Parse("http://localhost:8428")
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			// no need to parse form here as it was already parsed
			values := r.In.Form
			// add forecast time label to query
			// simplify and rename location as well
			tagFmt := `%s="%s"`
			loc := fmt.Sprintf(tagFmt, "location", params.Location.Name)
			src := fmt.Sprintf(tagFmt, "source", params.Source)
			fts := time.Now().Add(-5 * time.Minute).Truncate(time.Hour).Format(ForecastTimeFormat)
			ft := fmt.Sprintf(tagFmt, "forecast_time", fts)
			query := fmt.Sprintf("%s{%s,%s,%s}", params.Metric, loc, src, ft)
			//query := fmt.Sprintf()
			values.Set("query", query)
			body := values.Encode()
			r.Out.ContentLength = int64(len(body))
			r.Out.Body = io.NopCloser(strings.NewReader(body))
		},
	}
	proxy.ServeHTTP(resp, req)
}

// ParsedQuery is the information parsed and looked up from the prometheus query string
type ParsedQuery struct {
	Metric   string
	Location Location
	Source   string
	AdHoc    bool
}

// Params are the timestamps of the query range along with the query string information.
type Params struct {
	Start int64
	End   int64
	Step  int64
	ParsedQuery
}

// String implements Stringer by printing the parsed prometheus query string.
func (p Params) String() string {
	return fmt.Sprintf("Metric:%s Location:%s Source:%s Adhoc:%t\n", p.Metric, p.Location.Name, p.Source, p.AdHoc)
}

var queryRE = regexp.MustCompile(`(\w+)\{(.+)\}`)
var tagRE = regexp.MustCompile(`(\w+)="([^"]+)",?`)

// ParseQuery parses the information in the prometheus query string.
func (s *Server) ParseQuery(query string) (*ParsedQuery, error) {
	matches := queryRE.FindStringSubmatch(query)
	if len(matches) == 0 {
		return nil, errors.New("no matches found")
	}
	pq := &ParsedQuery{
		Metric: matches[1],
	}
	if strings.Index(pq.Metric, s.ConfigService.Config.Forecast.MeasurementName) != 0 &&
		strings.Index(pq.Metric, s.ConfigService.Config.Astronomy.MeasurementName) != 0 {
		return nil, fmt.Errorf("invalid metric name: %s", pq.Metric)
	}

	tagMatches := tagRE.FindAllStringSubmatch(matches[2], -1)
	tags := make(map[string]string)
	for _, tagMatch := range tagMatches {
		tags[tagMatch[1]] = tagMatch[2]
	}
	adhoc := false
	loc, ok := tags["locationTxt"]
	if !ok {
		if loc, ok = tags["locationAdhoc"]; !ok {
			return nil, errors.New("no location tag found")
		}
		adhoc = true
	}
	location, err := s.LocationService.ParseLocation(loc)
	if err != nil {
		return nil, err
	}
	pq.Location = *location
	pq.AdHoc = adhoc
	if pq.Source, ok = tags["source"]; !ok {
		return nil, errors.New("no source tag found")
	}
	return pq, nil
}

// ParseParams parses all of the information needed from the prometheus request.
func (s *Server) ParseParams(req *http.Request) (*Params, error) {
	err := req.ParseForm()
	if err != nil {
		return nil, err
	}
	pq, err := s.ParseQuery(req.Form.Get("query"))
	if err != nil {
		return nil, err
	}
	// fixme: <rfc3339 | unix_timestamp>
	//  error
	start, _ := strconv.ParseInt(req.Form.Get("start"), 10, 64)
	end, _ := strconv.ParseInt(req.Form.Get("end"), 10, 64)
	// fixme: <duration | float>
	//  error
	step, _ := strconv.ParseInt(req.Form.Get("step"), 10, 64)
	return &Params{
		Start:       start,
		End:         end,
		Step:        step,
		ParsedQuery: *pq,
	}, nil
}
