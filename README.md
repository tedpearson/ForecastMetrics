# ForecastMetrics

ForecastMetrics is a tool to store forecast data from multiple
sources in VictoriaMetrics or InfluxDB.

I currently use [VictoriaMetrics](https://victoriametrics.com) as my time series database.
Because of that, this project does a few things specificly to support it:
- Uses the influx 1.x Go client (VictoriaMetrics only supports Basic auth, not Token)
- Writes hourly sunup information
- Uses no retention policies (not supported in VictoriaMetrics)
- Every forecast is a new tag/label, since VictoriaMetrics doesn't support overwriting
  metrics as Influx does.
- Past data is written one data point per hour, also because overwriting data is unsupported.

#### Currently supported sources:
- National Weather Service (NWS)
- VisualCrossing
- No other sources planned at this time, due to not meeting the below
criteria (7 day hourly forecast, reasonably priced or free)
- Open an issue if you find a worthy source!

## Usage:

### Install
- Download a binary from the latest [Release][release] if your architecture is available

      curl -O https://github.com/tedpearson/ForecastMetrics/releases/download/v2.3.1/forecastmetrics-linux-arm

- Make the binary executable

      chmod +x forecastmetrics-linux-arm

- If your architecture is not avaialable, you'll need to build from source:
  - Clone this repo
  - [Install Go][install-go]
  -
        cd ForecastMetrics
        go build

### Configure

- Get the example config

      curl https://raw.githubusercontent.com/tedpearson/ForecastMetrics/master/config/forecastmetrics.example.yaml > forecastmetrics.yaml

- Modify the config with your own values for:
  - location(s)
  - influxdb connection
  - desired influx measurement names
  - which weather sources to enable
  - add your own key for limited access/pay sources

- Place the config file either in the same directory with forecastmetrics, in `/usr/local/etc/`, or
  in a `config` directory next to forecastmetrics.
      

### Run
There are no command line options, so just run the binary like this:

    ./forecastmetrics

## Grafana Dashboard
I've included my [grafana dashboard definition](grafana/dashboard.json) in the repo. 
Here is a screenshot of what it looks like when configured correctly.
I use this dashboard daily for my local weather forecast.
![grafana dashboard](grafana/dashboard.png)

## Rationale behind included/planned sources:
I was looking for a replacement for DarkSky, who were bought by
Apple and will be retiring their API at the end of <s>2021</s>2022.
DarkSky had the best forecasts and a generous free version,
with 7 days of forecast data available.

I used the DarkSky data to power my own visualizations of my
local forecast in Grafana. I find my Grafana graphs of forecast
data much more intuitive than any weather app or website out there.
I display the 7 day forecast for temps, precip, wind, and clouds,
on the same graph with 7 days of actual data history from my
Ambient Weather personal weather station, and also the forecast
from 24 hours previous.

So when I went looking for replacements I needed these features:
- At least 7 days of HOURLY forecast data. Daily highs and lows
are not very interesting to look at in a graph.
- I preferred Free APIs or APIs allowing at least 1500 forecasts
per month, as I only made <200 calls/day to DarkSky, and paying
large amounts for my personal forecast dashboard is just silly.
    - This is why visualcrossing is a supported source,
    because their free tier supports 250 forecasts/day.

[release]: https://github.com/tedpearson/ForecastMetrics/releases
[config-example]: https://github.com/tedpearson/ForecastMetrics/blob/master/config/forecastmetrics.example.yaml
[install-go]: https://golang.org/dl/