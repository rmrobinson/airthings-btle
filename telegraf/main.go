package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rmrobinson/airthings-btle"
	"tinygo.org/x/bluetooth"
)

var (
	serialNumber = flag.Int("serial", 0, "The serial number of the sensor to search for")
	adapterAddr  = flag.String("adapter", "", "The HCI adapter to use (i.e. hci0, hci1)")
)

func main() {
	flag.Parse()

	if *serialNumber < 1 {
		os.Exit(1)
	}

	btAdapter := bluetooth.DefaultAdapter

	if adapterAddr != nil && len(*adapterAddr) > 0 {
		btAdapter = bluetooth.NewAdapter(*adapterAddr)
	}

	err := btAdapter.Enable()
	if err != nil {
		os.Exit(1)
	}

	scanner := airthings.NewScanner(btAdapter)

	s, err := scanner.FindSensor(context.Background(), *serialNumber)
	if err != nil {
		os.Exit(1)
	}
	if s == nil {
		os.Exit(1)
	}

	if err := s.Refresh(context.Background()); err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	if err := s.RefreshHistory(ctx, 24); err != nil {
		os.Exit(1)
	}

	s.Disconnect()

	tags := fmt.Sprintf("serial_number=%d", *serialNumber)

	fields := fmt.Sprintf("battery_level=%.1f,rssi=%di", s.BatteryLevel(), s.RSSI())
	fmt.Printf("sensor,%s %s %d\n", tags, fields, time.Now().UnixNano())

	for _, h := range s.HistoricalMeasurements() {
		fields := fmt.Sprintf("radon_short_term_avg=%.1f,radon_long_term_avg=%.1f", h.RadonShortTermAvg, h.RadonLongTermAvg)
		fmt.Printf("air_quality,%s %s %d\n", tags, fields, h.Timestamp.UnixNano())

		for i := 0; i < len(h.Temperature); i++ {
			fields := fmt.Sprintf("humidity=%.1f,illuminance=%.1f,temperature=%.1f,relative_atmospheric_pressure=%.1f,co2_level=%.1f,voc_level=%.1f", h.Humidity[i], h.Illuminance[i], h.Temperature[i], h.RelativeAtmosphericPressure[i], h.CO2Level[i], h.VOCLevel[i])
			fmt.Printf("air_quality,%s %s %d\n", tags, fields, h.Timestamp.Add(time.Duration(i*5)*time.Minute).UnixNano())
		}
	}
}
