package convert

import (
	"fmt"
	"math"
	"strconv"
)

func CToF(celsius float64) float64 {
	return Round(celsius*9/5+32, 2)
}

func FToC(fahrenheit float64) float64 {
	return (fahrenheit - 32) * 5 / 9
}

func Identity(a float64) float64 {
	return a
}

func KmhToMph(kmh float64) float64 {
	return Round(kmh*0.6213711922, 2)
}

func PercentToRatio(percent float64) float64 {
	return Round(percent/100, 3)
}

func MmToIn(mm float64) float64 {
	return Round(mm/25.4, 4)
}

func Round(a float64, digits int) float64 {
	if digits > 10 || digits < 0 {
		panic("Round() only supports 0-10 digits")
	}
	scaler := math.Pow(10, float64(digits))
	return math.Round(a*scaler) / scaler
}

func NilToZero(a *float64) *float64 {
	if a == nil {
		zero := 0.0
		return &zero
	}
	return a
}

func StrToF(s string, msg string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fmt.Printf("Unable to parse %s value '%s'\n", msg, s)
		v = 0
	}
	return v
}
