package main

import "fmt"

func main() {
	fmt.Printf("hello world\n")
	GetWeather("38.7541","-77.3454")
}


// todo:
	// noaa api
	// visualcrossing (free version for now)
	// theglobalweather (test with free(?), or sign up, cheap per call)

	// authentication as needed
	// make api call[s] and get results

	// create influxdb schema that supports all 3 options, in a way that is easily queryable and supports dashboards
	// integrate correctly with golang influx client
	// write data to influx correctly

	// decide on running this program on a timer in systemd or having a timer in the process
	// currently leaning towards systemd timer.