package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Server provides the promethus endpoint for ForecastMetrics.
type Server struct {
	LocationService    LocationService
	Dispatcher         *Dispatcher
	PromConverter      PromConverter
	AuthToken          string
	AllowedMetricNames []string
}

// Start starts the prometheus endpoint.
func (s *Server) Start(port int64) {
	// don't 404 on other prometheus endpoints
	http.HandleFunc("/api/v1/", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(204)
	})
	http.Handle("/api/v1/query_range", s)
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
	err := req.ParseForm()
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Printf("Failed to parse form: %s", err)
	}
	params, err := s.ParseParams(req.Form)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	forecast, err := s.Dispatcher.GetForecast(params.Location, params.Source, params.AdHoc)
	if err != nil {
		fmt.Printf("Error getting forecast: %+v\n", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	// convert to prometheus response.
	promResponse := s.PromConverter.ConvertToTimeSeries(*forecast, *params)
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

var queryRE = regexp.MustCompile(`^(\w+)\{(.+)}$`)
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
	validMetric := slices.ContainsFunc(s.AllowedMetricNames, func(str string) bool {
		return strings.HasPrefix(pq.Metric, str)
	})
	if !validMetric {
		return nil, fmt.Errorf("invalid metric name: %s", pq.Metric)
	}

	tagMatches := tagRE.FindAllStringSubmatch(matches[2], -1)
	tags := make(map[string]string)
	for _, tagMatch := range tagMatches {
		tags[tagMatch[1]] = tagMatch[2]
	}
	adhoc := true
	if save := tags["save"]; save == "true" {
		adhoc = false
	}
	loc, ok := tags["location"]
	if !ok {
		return nil, errors.New("no location tag found")
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

// ParseParams parses all the information needed from the prometheus request.
func (s *Server) ParseParams(Form url.Values) (*Params, error) {
	pq, err := s.ParseQuery(Form.Get("query"))
	if err != nil {
		return nil, err
	}
	// fixme: <rfc3339 | unix_timestamp>
	//  error
	start, _ := strconv.ParseInt(Form.Get("start"), 10, 64)
	end, _ := strconv.ParseInt(Form.Get("end"), 10, 64)
	// fixme: <duration | float>
	//  error
	step, _ := strconv.ParseInt(Form.Get("step"), 10, 64)
	return &Params{
		Start:       start,
		End:         end,
		Step:        step,
		ParsedQuery: *pq,
	}, nil
}
