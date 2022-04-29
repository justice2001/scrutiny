package database

import (
	"context"
	"fmt"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"time"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Temperature Data
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (sr *scrutinyRepository) SaveSmartTemperature(ctx context.Context, wwn string, deviceProtocol string, collectorSmartData collector.SmartInfo) error {
	if len(collectorSmartData.AtaSctTemperatureHistory.Table) > 0 {

		for ndx, temp := range collectorSmartData.AtaSctTemperatureHistory.Table {

			minutesOffset := collectorSmartData.AtaSctTemperatureHistory.LoggingIntervalMinutes * int64(ndx) * 60
			smartTemp := measurements.SmartTemperature{
				Date: time.Unix(collectorSmartData.LocalTime.TimeT-minutesOffset, 0),
				Temp: temp,
			}

			tags, fields := smartTemp.Flatten()
			tags["device_wwn"] = wwn
			p := influxdb2.NewPoint("temp",
				tags,
				fields,
				smartTemp.Date)
			err := sr.influxWriteApi.WritePoint(ctx, p)
			if err != nil {
				return err
			}
		}
		// also add the current temperature.
	} else {

		smartTemp := measurements.SmartTemperature{
			Date: time.Unix(collectorSmartData.LocalTime.TimeT, 0),
			Temp: collectorSmartData.Temperature.Current,
		}

		tags, fields := smartTemp.Flatten()
		tags["device_wwn"] = wwn
		p := influxdb2.NewPoint("temp",
			tags,
			fields,
			smartTemp.Date)
		return sr.influxWriteApi.WritePoint(ctx, p)
	}
	return nil
}

func (sr *scrutinyRepository) GetSmartTemperatureHistory(ctx context.Context, durationKey string) (map[string][]measurements.SmartTemperature, error) {
	//we can get temp history for "week", "month", DURATION_KEY_YEAR, "forever"

	deviceTempHistory := map[string][]measurements.SmartTemperature{}

	//TODO: change the query range to a variable.
	queryStr := sr.aggregateTempQuery(durationKey)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err == nil {
		// Use Next() to iterate over query result lines
		for result.Next() {

			if deviceWWN, ok := result.Record().Values()["device_wwn"]; ok {

				//check if deviceWWN has been seen and initialized already
				if _, ok := deviceTempHistory[deviceWWN.(string)]; !ok {
					deviceTempHistory[deviceWWN.(string)] = []measurements.SmartTemperature{}
				}

				currentTempHistory := deviceTempHistory[deviceWWN.(string)]
				smartTemp := measurements.SmartTemperature{}

				for key, val := range result.Record().Values() {
					smartTemp.Inflate(key, val)
				}
				smartTemp.Date = result.Record().Values()["_time"].(time.Time)
				currentTempHistory = append(currentTempHistory, smartTemp)
				deviceTempHistory[deviceWWN.(string)] = currentTempHistory
			}
		}
		if result.Err() != nil {
			fmt.Printf("Query error: %s\n", result.Err().Error())
		}
	} else {
		return nil, err
	}
	return deviceTempHistory, nil

}
