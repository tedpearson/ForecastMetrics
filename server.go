package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"strconv"
)

type Server struct {
	LocationService LocationService
	Dispatcher      *Dispatcher
	Prometheus      Prometheus
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
	rs, err := httputil.DumpRequest(req, true)
	if err == nil {
		//log.Println(string(rs))
		_ = rs
		fmt.Printf("Got request.\n")
	}

	// get params
	params, err := s.ParseParams(req)
	// todo handle error to client

	//loc, err := s.LocationService.ParseLocation("Boise, ID")
	//if err != nil {
	//	panic(err)
	//}
	forecast, err := s.Dispatcher.GetForecast(params.Location, params.Source, params.AdHoc)
	// convert to prometheus response.
	promResponse := ConvertToTimeSeries(*forecast, params.Metric, params.Start, params.End, params.Step)
	// send prom response as json to client
	resp.Header().Add("content-type", "application/json")
	respJson, err := json.Marshal(promResponse)
	// todo: handle err to client.
	_, err = resp.Write(respJson)
	// todo: handle err to client.
	//_, err = resp.Write([]byte(fmt.Sprintf("%+v", loc)))
	//if err != nil {
	//	panic(err)
	//}
	//_, err = resp.Write([]byte(fmt.Sprintf("%+v", forecast)))
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

func (s *Server) ParseParams(req *http.Request) (*Params, error) {
	err := req.ParseForm()
	if err != nil {
		return nil, err
	}
	pq, err := s.Prometheus.ParseQuery(req.Form.Get("query"))
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
