package main

import (
	"encoding/json"
	"fmt"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

// talk to nws api and get results

func GetWeather(lat string, lon string) error {
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	cache := diskcache.New("/tmp/weather-cache")
	client := httpcache.NewTransport(cache).Client()
	//client := httpcache.NewMemoryCacheTransport().Client()
	body, err := makeRequest(url, client)
	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return err
	}
	gridpointUrl := jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string)

	// okay we have a gridpoint url. get it and turn it into an object and do fun things with it
	body, err = makeRequest(gridpointUrl, client)
	log.Println(string(body))
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return err
	}
	fmt.Printf("%v", jsonResponse)

	return nil
}

func makeRequest(url string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// user-agent required by weather.gov with email
	req.Header.Set("User-Agent", "weather2influxdb; by ted@tedpearson.com")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer cleanup(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func cleanup(closer io.Closer) {
	// todo: better error handling
	if closer.Close() != nil {
		log.Fatalln("Failed to cleanup")
	}
}