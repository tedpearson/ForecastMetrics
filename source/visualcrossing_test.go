package source

import (
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/tedpearson/weather2influxdb/http"
	"testing"
)

func TestVisualCrossing_GetWeather(t *testing.T) {
	v := VisualCrossing{}
	v.GetWeather("1", "2", http.Retryer{
		Client: httpcache.NewTransport(diskcache.New("/tmp/weather2influxdb-cache")).Client(),
	})
}