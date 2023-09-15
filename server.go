package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strconv"
)

type Server struct {
	LocationService LocationService
	Dispatcher      *Dispatcher
}

func (s *Server) Start(port int64) {
	//   creates prometheus converter
	//   creates name processor?
	//   needs forecast dispatcher
	http.Handle("/", s)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		panic(err)
	}
}

func (s *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// get params
	params, err := s.ParseParams(req)
	fmt.Printf("Request: %s", params)
	// todo handle error to client

	forecast, err := s.Dispatcher.GetForecast(params.Location, params.Source, params.AdHoc)
	// convert to prometheus response.
	promResponse := ConvertToTimeSeries(*forecast, *params)
	// send prom response as json to client
	resp.Header().Add("content-type", "application/json")
	respJson, err := json.Marshal(promResponse)
	// todo: handle err to client.
	_, err = resp.Write(respJson)
	// todo: handle err to client.
	if err != nil {
		panic(err)
	}
}

type Params struct {
	Start int64
	End   int64
	Step  int64
	ParsedQuery
}

func (p Params) String() string {
	return fmt.Sprintf("Metric:%s Location:%s Source:%s Adhoc:%t\n", p.Metric, p.Location.Name, p.Source, p.AdHoc)
}

// fixme: don't hard code metric names
var queryRE = regexp.MustCompile(`((?:forecast2_|astronomy_)\w+)\{(.+)\}`)
var tagRE = regexp.MustCompile(`(\w+)="([^"]+)",?`)

type ParsedQuery struct {
	Metric   string
	Location Location
	Source   string
	AdHoc    bool
}

func (s *Server) ParseQuery(query string) (*ParsedQuery, error) {
	matches := queryRE.FindStringSubmatch(query)
	if len(matches) == 0 {
		return nil, errors.New("no matches found")
	}
	pq := &ParsedQuery{
		Metric: matches[1],
	}
	// fixme: this doesn't work, won't match locs with comma.
	tagMatches := tagRE.FindAllStringSubmatch(matches[2], -1)
	for _, tagMatch := range tagMatches {
		switch tagMatch[1] {
		case "locationAdhoc":
			pq.AdHoc = true
			fallthrough
		case "locationTxt":
			location, err := s.LocationService.ParseLocation(tagMatch[2])
			if err != nil {
				return nil, err
			}
			pq.Location = *location
		case "source":
			pq.Source = tagMatch[2]
			continue
		default:
			// unknown tag, ignore.
			continue
		}
	}
	return pq, nil
}

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
