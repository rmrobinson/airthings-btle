package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/rmrobinson/airthings-btle"
	"tinygo.org/x/bluetooth"
)

var (
	serialNumber = flag.Int("serial", 0, "The serial number of the sensor to search for")
	adapterAddr  = flag.String("adapter", "", "The HCI adapter to use (i.e. hci0, hci1)")
	action       = flag.String("action", "get", "Possible actions. \"get\" is default, to get the data. \"discover\" will list available UUIDs")
	history      = flag.Int("history", 0, "Number of historical records to retrieve when using the \"get\" action. Won't get history if not specified or < 1.")
)

func main() {
	flag.Parse()

	if *serialNumber < 1 {
		log.Fatal("must supply a serial number to scan for")
	}

	btAdapter := bluetooth.DefaultAdapter

	if adapterAddr != nil && len(*adapterAddr) > 0 {
		btAdapter = bluetooth.NewAdapter(*adapterAddr)
	}

	err := btAdapter.Enable()
	if err != nil {
		log.Fatal("unable to enable bt adapter", err)
	}

	scanner := airthings.NewScanner(btAdapter)

	log.Printf("scanning for %d\n", *serialNumber)
	s, err := scanner.FindSensor(context.Background(), *serialNumber)
	if err != nil {
		log.Fatalf("error finding sensor: %s\n", err.Error())
		return
	}
	if s == nil {
		log.Print("no sensor found\n")
		return
	}

	if *action == "get" {
		log.Printf("getting updated values for %d\n", *serialNumber)
		err = s.Refresh(context.Background())
		if err != nil {
			log.Printf("unable to refresh sensor: %s", err)
			return
		}

		if *history > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			err = s.RefreshHistory(ctx, *history)
			if err != nil {
				log.Printf("unable to refresh history: %s", err)
				return
			}
		}

		s.Disconnect()

		log.Printf("Humidity|Illuminance|Radon (Short Term Avg)|Radon (Long Term Avg)|Temperature|Rel Atm Pressure|CO2 Level|VOC Level|")
		m := s.CurrentMeasurement()
		log.Printf("%.1f %%rH %0.1f %% %.1f Bq/m3 %.1f Bq/m3 %.1f degC %.1f hPa %.1f ppm %.1f ppb\n",
			m.Humidity, m.Illuminance, m.RadonShortTermAvg, m.RadonLongTermAvg,
			m.Temperature, m.RelativeAtmosphericPressure, m.CO2Level, m.VOCLevel)
		log.Printf("Battery: %.1f%%, RSSI: %d dBm\n", s.BatteryLevel(), s.RSSI())

		if len(s.HistoricalMeasurements()) > 0 {
			log.Printf("Historical measurements:\n")
			for _, h := range s.HistoricalMeasurements() {
				log.Printf("  %s: Radon (short term): %.1f Bq/m3, Radon (long term): %.1f Bq/m3\n", h.Timestamp, h.RadonShortTermAvg, h.RadonLongTermAvg)
				log.Printf("    Temperature, Humidity, Rel Atm Pressure, CO2 Level, VOC Level\n")
				for i := 0; i < len(h.Temperature); i++ {
					log.Printf("    %.1f degC, %.1f %%rH, %.1f hPa, %.1f ppm, %.1f ppb\n", h.Temperature[i], h.Humidity[i], h.RelativeAtmosphericPressure[i], h.CO2Level[i], h.VOCLevel[i])
				}
			}
		}
	} else if *action == "discover" {
		log.Printf("discovering UUIDs for %d\n", *serialNumber)
		svcList, err := s.GetDeviceProfile()
		if err != nil {
			log.Printf("unable to discover UUIDs: %s", err)
			return
		}

		s.Disconnect()

		for _, svc := range svcList {
			log.Printf("svc UUID: %s\n", svc.ServiceUUID)
			for _, char := range svc.CharacteristicsUUIDs {
				log.Printf("  char UUID: %s\n", char)
			}
		}
	}
}
