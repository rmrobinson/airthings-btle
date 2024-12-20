package main

import (
	"context"
	"log"
	"time"

	"github.com/rmrobinson/airthings"
	"tinygo.org/x/bluetooth"
)

func main() {
	btAdapter := bluetooth.DefaultAdapter

	err := btAdapter.Enable()
	if err != nil {
		log.Fatal("unable to enable bt adapter", err)
	}

	scanner := airthings.NewScanner(btAdapter)

	log.Printf("scanning\n")
	s, err := scanner.FindSensor(context.Background(), 2930170133)
	if err != nil {
		log.Fatalf("unable to find sensor: %s\n", err.Error())
	}
	if s == nil {
		log.Fatal("unable to find sensor\n")
	}

	log.Printf("Humidity|Illuminance|Radon (Short Term Avg)|Radon (Long Term Avg)|Temperature|Rel Atm Pressure|CO2 Level|VOC Level|")
	log.Printf("%.1f %%rH %0.1f %% %.1f Bq/m3 %.1f Bq/m3 %.1f degC %.1f hPa %.1f ppm %.1f ppb\n", s.Humidity, s.Illuminance, s.RadonShortTermAvg, s.RadonLongTermAvg, s.Temperature, s.RelativeAtmosphericPressure, s.CO2Level, s.VOCLevel)

	ticker := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-ticker.C:
			err = s.Refresh()
			if err != nil {
				log.Printf("unable to refresh: %s\n", err.Error())
				continue
			}
			log.Printf("%.1f %%rH %0.1f %% %.1f Bq/m3 %.1f Bq/m3 %.1f degC %.1f hPa %.1f ppm %.1f ppb\n", s.Humidity, s.Illuminance, s.RadonShortTermAvg, s.RadonLongTermAvg, s.Temperature, s.RelativeAtmosphericPressure, s.CO2Level, s.VOCLevel)
		}
	}
}
