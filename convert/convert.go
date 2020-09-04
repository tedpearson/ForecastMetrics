package convert

import "math"

func CToF(celsius float64) float64 {
	return math.Round((celsius*9/5+32)*100) / 100
}

func FToC(fahrenheit float64) float64 {
	return (fahrenheit - 32) * 5 / 9
}

func Identity(a float64) float64 {
	return a
}

func KmhToMph(kmh float64) float64 {
	return math.Round(kmh*0.6213711922*100) / 100
}

func PercentToRatio(percent float64) float64 {
	return math.Round(percent * 10) / 1000
}

func MmToIn(mm float64) float64 {
	return math.Round(mm/25.4*10000) / 10000
}

func NilToZero(a *float64) float64 {
	if a == nil {
		return 0
	}
	return *a
}
