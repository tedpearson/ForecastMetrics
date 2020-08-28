package main

import "time"

type ForecastRecord struct {
	Time time.Time
	Temperature float64
	Dewpoint float64
	FeelsLike float64
	SkyCover int
	WindDirection int
	WindSpeed float64
	WindGust float64
	PrecipitationProbability int
	PrecipitationAmount float64
	// maybe:
	// SnowAmount float64
	// IceAmount float64
}