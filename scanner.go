package airthings

import (
	"context"
	"errors"
	"log"

	"tinygo.org/x/bluetooth"
)

var (
	// ErrInvalidBluetoothAdapter is returned if the supplied Bluetooth adapter isn't able to be used.
	ErrInvalidBluetoothAdapter = errors.New("invalid bluetooth adaptor suppled")
)

type pendingSensor struct {
	scanResult   bluetooth.ScanResult
	serialNumber int
}

// Scanner allows for discovery of an Airthings Wave Plus sensor broadcasting over Bluteooth
type Scanner struct {
	adapter *bluetooth.Adapter
}

// NewScanner creates a new scanner using the provided Bluetooth adapter to search for the sensor.
func NewScanner(adapter *bluetooth.Adapter) *Scanner {
	return &Scanner{
		adapter: adapter,
	}
}

// FindSensor looks for an Airthings sensor using the provided serial number.
func (s *Scanner) FindSensor(ctx context.Context, serialNumber int) (*Sensor, error) {
	if s.adapter == nil {
		return nil, ErrInvalidBluetoothAdapter
	}

	res := make(chan pendingSensor, 1)

	err := s.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		// Check if this scan result matches the specified AirThings serial number
		for _, mde := range result.ManufacturerData() {
			if mde.CompanyID == airthingsSerialNumberCompanyID {
				sn, err := ParseSerialNumber(mde.Data)
				if err != nil {
					log.Printf("found Airthings BT device but couldn't parse serial number: %s\n", err.Error())
					return
				}

				if sn != serialNumber {
					continue
				}

				//log.Printf("found sensor %d\n", sn)
				s.adapter.StopScan()

				select {
				case res <- pendingSensor{
					scanResult:   result,
					serialNumber: sn,
				}:
				default:
				}
			}
		}
	})

	if err != nil {
		//log.Printf("unable to scan: %s\n", err.Error())
		return nil, err
	}

	ps := <-res
	//log.Printf("connecting to %d using Bluetooth address %s\n", ps.serialNumber, ps.scanResult.Address.String())
	device, err := s.adapter.Connect(ps.scanResult.Address, bluetooth.ConnectionParams{})
	if err != nil {
		//log.Fatalf("unable to connect to device: %s", err)
		return nil, err
	}

	//log.Printf("connected to %d creating sensor\n", ps.serialNumber)
	sensor := NewSensor(ps.serialNumber, device)

	return sensor, nil
}
