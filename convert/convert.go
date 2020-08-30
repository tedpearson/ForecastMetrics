package convert

import "math"

func CToF(celsius float64) float64 {
	return celsius*9/5 + 32
}

func Identity(a float64) float64 {
	return a
}

func KmhToMph(kmh float64) float64 {
	return math.Round(kmh*0.6213711922*100) / 100
}

func PercentToRatio(percent float64) float64 {
	return percent / 100
}

func MmToIn(mm float64) float64 {
	return math.Round(mm/25.4*10000) / 10000
}