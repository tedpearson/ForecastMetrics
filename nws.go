package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// talk to nws api and get results

func GetWeather(lat string, lon string) {
	// include user agent in request with email.
	// find gridpoint
	url := fmt.Sprintf("https://api.weather.gov/points/%s,%s", lat, lon)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("User-Agent", "weather2influxdb; by ted@tedpearson.com")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(string(body))
	var jsonResponse map[string]interface{}
	json.Unmarshal(body, &jsonResponse)
	log.Printf("%v", jsonResponse)
	log.Printf(jsonResponse["properties"].(map[string]interface{})["forecastGridData"].(string))

	client.Get(url)
	// lookup gridpoint
	// use if-modified-since headers to not overwhelm system
}