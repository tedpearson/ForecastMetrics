package main

import (
	"fmt"
	"github.com/go-errors/errors"
)

func main() {
	err := GetWeather("38.7541","-77.3454")
	if err != nil {
		// todo: only call ErrorStack if it's an errors error. Look at docs.
		fmt.Println(err.(*errors.Error).ErrorStack())
	}
}


// todo:
	// noaa api
		// get gridpoint
		// get weather
		// convert into normalized struct
	// visualcrossing (free version for now)
	// theglobalweather (test with free(?), or sign up, cheap per call)

	// authentication as needed
	// make api call[s] and get results

	// create influxdb schema that supports all 3 options, in a way that is easily queryable and supports dashboards
	// integrate correctly with golang influx client
	// write data to influx correctly

	// decide on running this program on a timer in systemd or having a timer in the process
	// currently leaning towards systemd timer.