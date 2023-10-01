package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

var latLonRe = regexp.MustCompile(`(\d+\.\d+),\s*(\d+\.\d+)`)

// LocationService parses strings into Location, using the Bing Maps Locations API.
type LocationService struct {
	BingToken string
}

// ParseLocation turns strings into Locations
// allowed formats:
// lat,lon (name is blank)
// lat,lon|name
// city, state
// city, state|name
func (l LocationService) ParseLocation(s string) (*Location, error) {
	parts := strings.Split(s, "|")
	loc := parts[0]
	var name string
	if len(parts) > 1 {
		name = parts[1]
	}
	m := latLonRe.FindStringSubmatch(loc)
	if m != nil {
		return &Location{
			Name:      name,
			Latitude:  m[1],
			Longitude: m[2],
		}, nil
	}
	location := &Location{Name: name}
	err := l.lookup(loc, location)
	if err != nil {
		return nil, err
	}
	return location, nil
}

// lookup fills out the location argument with information looked up from
// the Bing Maps Locations API. The Name field on Location will only be populated
// if it is an empty string.
func (l LocationService) lookup(s string, location *Location) error {
	q := url.Values{}
	q.Add("q", s)
	q.Add("key", l.BingToken)
	resp, err := http.Get("http://dev.virtualearth.net/REST/v1/Locations?" + q.Encode())
	if err != nil {
		fmt.Printf("Failed to look up %s\n", s)
		return err
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		fmt.Printf("Failed to copy from response\n")
		return err
	}
	val, err := fastjson.ParseBytes(buf.Bytes())
	if err != nil {
		fmt.Printf("Failed to parse json %s\n", buf.String())
		return err
	}
	record := val.Get("resourceSets", "0", "resources", "0")
	coords := record.GetArray("point", "coordinates")
	if record == nil || coords == nil {
		return fmt.Errorf("failed to look up location '%s'", s)
	}
	latF, err := coords[0].Float64()
	if err != nil {
		fmt.Printf("Failed to get coordinates from json %s\n", buf.String())
		return err
	}
	lonF, err := coords[1].Float64()
	if err != nil {
		fmt.Printf("Failed to get coordinates from json %s\n", buf.String())
		return err
	}
	if location.Name == "" {
		name := record.GetStringBytes("name")
		if name == nil {
			fmt.Printf("Failed to get name from json %s\n", buf.String())
			return err
		}
		location.Name = string(name)
	}
	location.Latitude = strconv.FormatFloat(latF, 'f', -1, 64)
	location.Longitude = strconv.FormatFloat(lonF, 'f', -1, 64)
	return nil
}
