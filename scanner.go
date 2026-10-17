package airthings

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	sensorTypeWavePlus = iota

	// stopScanRetryInterval is how often a cancelled FindSensor re-issues StopScan until the scan has
	// actually ended. The first attempt can land before the adapter has registered the scan, in which
	// case it is a no-op and the scan would otherwise run on.
	stopScanRetryInterval = 100 * time.Millisecond
)

var (
	// ErrInvalidBluetoothAdapter is returned if the supplied Bluetooth adapter isn't able to be used.
	ErrInvalidBluetoothAdapter = errors.New("invalid bluetooth adaptor suppled")

	// ErrSensorNotFound is returned if the scan ended without hearing the requested sensor.
	ErrSensorNotFound = errors.New("sensor not found")
)

// bluetoothAdapter is the part of *bluetooth.Adapter the scanner uses, split out so the scan
// lifecycle can be tested without Bluetooth hardware.
type bluetoothAdapter interface {
	Scan(callback func(*bluetooth.Adapter, bluetooth.ScanResult)) error
	StopScan() error
	Connect(address bluetooth.Address, params bluetooth.ConnectionParams) (bluetooth.Device, error)
}

type pendingSensor struct {
	scanResult   bluetooth.ScanResult
	serialNumber int
	sensorType   int
}

// Scanner allows for discovery of an Airthings Wave Plus sensor broadcasting over Bluteooth
type Scanner struct {
	adapter bluetoothAdapter
}

// NewScanner creates a new scanner using the provided Bluetooth adapter to search for the sensor.
func NewScanner(adapter *bluetooth.Adapter) *Scanner {
	// A nil *bluetooth.Adapter must stay a nil interface so FindSensor's check below sees it.
	if adapter == nil {
		return &Scanner{}
	}
	return &Scanner{
		adapter: adapter,
	}
}

// FindSensor looks for an Airthings sensor using the provided serial number.
//
// The underlying scan only ends when it is stopped, which happens on a match. If the sensor is not
// heard, FindSensor therefore keeps scanning until ctx ends and then returns ctx's error, so the
// caller must supply a ctx with a deadline or cancellation, or an unheard sensor blocks it
// indefinitely. ErrSensorNotFound is returned if the scan ends for any other reason without a match.
func (s *Scanner) FindSensor(ctx context.Context, serialNumber int) (Sensor, error) {
	if s.adapter == nil {
		return nil, ErrInvalidBluetoothAdapter
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	res := make(chan pendingSensor, 1)

	// StopScan isn't safe to call from two goroutines at once (it can close the same channel twice),
	// and both the scan callback and the cancellation watcher below call it.
	var stopMu sync.Mutex
	stopScan := func() {
		stopMu.Lock()
		defer stopMu.Unlock()
		_ = s.adapter.StopScan()
	}

	// Scan blocks until the scan is stopped, so end it when ctx does; otherwise a sensor that is
	// never heard would hold this call (and whatever lock the caller holds) forever.
	scanDone := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-scanDone:
			return
		case <-ctx.Done():
		}
		for {
			stopScan()
			select {
			case <-scanDone:
				return
			case <-time.After(stopScanRetryInterval):
			}
		}
	}()

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
				var sensorType int
				if result.HasServiceUUID(serviceUUIDWavePlusData) {
					sensorType = sensorTypeWavePlus
				}

				//log.Printf("found sensor %d\n", sn)
				stopScan()

				select {
				case res <- pendingSensor{
					scanResult:   result,
					serialNumber: sn,
					sensorType:   sensorType,
				}:
				default:
				}
			}
		}
	})
	close(scanDone)
	<-watcherDone

	if err != nil {
		//log.Printf("unable to scan: %s\n", err.Error())
		return nil, err
	}

	var ps pendingSensor
	select {
	case ps = <-res:
	default:
		// The scan ended without a match: it was cancelled, or stopped by something else.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, ErrSensorNotFound
	}

	//log.Printf("connecting to %d using Bluetooth address %s\n", ps.serialNumber, ps.scanResult.Address.String())
	device, err := s.adapter.Connect(ps.scanResult.Address, bluetooth.ConnectionParams{})
	if err != nil {
		//log.Fatalf("unable to connect to device: %s", err)
		return nil, err
	}

	//log.Printf("connected to %d creating sensor\n", ps.serialNumber)
	if ps.sensorType == sensorTypeWavePlus {
		return newWavePlusSensor(ps.serialNumber, device, int(ps.scanResult.RSSI)), nil
	} else {
		return nil, errors.New("unsupported sensor type found")
	}
}
