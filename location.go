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

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/valyala/fastjson"
)

var latLonRe = regexp.MustCompile(`(\d+\.\d+),\s*(\d+\.\d+)`)

type LocationResult struct {
	Location *Location
	Error    error
}

// LocationService parses strings into Location, using the Azure Maps Get Geocoding API.
type LocationService struct {
	AzureSharedKey string
	cache          *cache.Cache[string, LocationResult]
}

// ParseLocation gets the cached location or delegates to parseLocation
func (l LocationService) ParseLocation(s string) (*Location, error) {
	if item, ok := l.cache.Get(s); ok {
		return item.Location, item.Error
	}
	loc, err := l.parseLocation(s)
	l.cache.Set(s, LocationResult{loc, err})
	return loc, err
}

// parseLocation turns strings into Locations
// allowed formats:
// lat,lon (name is blank)
// lat,lon|name
// city, state
// city, state|name
func (l LocationService) parseLocation(s string) (*Location, error) {
	parts := strings.Split(s, "|")
	loc := strings.ReplaceAll(parts[0], "\n", "")
	loc = strings.ReplaceAll(loc, "\r", "")
	var name string
	if len(parts) > 1 {
		name = strings.ReplaceAll(parts[1], "\n", "")
		name = strings.ReplaceAll(name, "\r", "")
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
// the Azure Maps Get Geocoding API. The Name field on Location will only be populated
// if it is an empty string.
func (l LocationService) lookup(s string, location *Location) error {
	q := url.Values{}
	q.Add("api-version", "2023-06-01")
	q.Add("query", s)
	q.Add("subscription-key", l.AzureSharedKey)
	resp, err := http.Get("https://atlas.microsoft.com/geocode?" + q.Encode())
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
	errorCode := val.GetStringBytes("error", "code")
	errorMsg := val.GetStringBytes("error", "message")
	if errorMsg != nil {
		return fmt.Errorf("failed to look up location '%s', error: %s, %s", s, string(errorCode), string(errorMsg))
	}
	record := val.Get("features", "0")
	coords := record.GetArray("geometry", "coordinates")
	if record == nil || coords == nil {
		return fmt.Errorf("failed to look up location '%s'", s)
	}
	latF, err := coords[1].Float64()
	if err != nil {
		fmt.Printf("Failed to get coordinates from json %s\n", buf.String())
		return err
	}
	lonF, err := coords[0].Float64()
	if err != nil {
		fmt.Printf("Failed to get coordinates from json %s\n", buf.String())
		return err
	}
	if location.Name == "" {
		name := record.GetStringBytes("properties", "address", "formattedAddress")
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
