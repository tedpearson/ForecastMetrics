package source

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStr2Times(t *testing.T) {
	var tests = []struct {
		dateString string
		expected   []time.Time
	}{
		{
			"2020-08-28T23:00:00+00:00/PT1H",
			[]time.Time{
				time.Date(2020, 8, 28, 23, 0, 0, 0, time.UTC),
			},
		},
		{
			"2020-08-28T17:00:00+00:00/PT4H",
			[]time.Time{
				time.Date(2020, 8, 28, 17, 0, 0, 0, time.UTC),
				time.Date(2020, 8, 28, 18, 0, 0, 0, time.UTC),
				time.Date(2020, 8, 28, 19, 0, 0, 0, time.UTC),
				time.Date(2020, 8, 28, 20, 0, 0, 0, time.UTC),
			},
		},
		{
			"2020-08-27T23:00:00+00:00/PT3H",
			[]time.Time{
				time.Date(2020, 8, 27, 23, 0, 0, 0, time.UTC),
				time.Date(2020, 8, 28, 0, 0, 0, 0, time.UTC),
				time.Date(2020, 8, 28, 1, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, testTable := range tests {
		t.Run(testTable.dateString, func(t *testing.T) {
			actual, err := durationStrToHours(testTable.dateString)
			assert.Nil(t, err)
			assert.Equal(t, testTable.expected, actual)
		})
	}
}
