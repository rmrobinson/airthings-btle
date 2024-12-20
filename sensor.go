package airthings

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"

	"tinygo.org/x/bluetooth"
)

// The Bluetooth characteristic UUID for Wave Plus measurement data.
var characteristicUUIDwavePlusData, _ = bluetooth.ParseUUID("b42e2a68-ade7-11e4-89d3-123b93f75cba")

// Sensor represents an instance of an Airthings sensor
type Sensor struct {
	device bluetooth.Device

	// SerialNumber is the serial number of this sensor
	SerialNumber int

	// Humidity represents the humidity that was last measured by the sensor, in % of relative humidity
	Humidity float32
	// Illuminance returns the light level, in %
	Illuminance float32
	// RadonShortTermAvg represents the short-term average of the radon measured, in Bq/m3
	RadonShortTermAvg float32
	// RadonLongTermAvg represents the long-term average of the radon measured, in Bq/m3
	RadonLongTermAvg float32
	// Temperature represents the temperature measured, in degrees C
	Temperature float32
	// RelativeAtmosphericPressure represents the relative pressure, in hPa
	RelativeAtmosphericPressure float32
	// CO2Level represents the measured CO2 level, in ppm
	CO2Level float32
	// VOCLevel represents the measured VOC level, in ppb
	VOCLevel float32
}

// NewSensor creates a sensor from a connected Bluetooth device which will be read from to retrieve values periodically.
func NewSensor(serialNumber int, device bluetooth.Device) *Sensor {
	return &Sensor{
		device:                      device,
		SerialNumber:                serialNumber,
		Humidity:                    -1,
		RadonShortTermAvg:           -1,
		RadonLongTermAvg:            -1,
		Temperature:                 -1,
		RelativeAtmosphericPressure: -1,
		CO2Level:                    -1,
		VOCLevel:                    -1,
	}
}

// Address returns the Bluetooth address of this sensor.
func (s *Sensor) Address() string {
	return s.device.Address.String()
}

// Refresh accesses the Bluetooth device to get the most recent readings for this device.
func (s *Sensor) Refresh() error {
	log.Printf("getting characteristics of device %s\n", s.device.Address.String())
	svcs, err := s.device.DiscoverServices(nil)
	if err != nil {
		return err
	}
	if len(svcs) < 1 {
		return errors.New("empty service list discovered")
	}

	log.Printf("discovering characteristics for service %s\n", svcs[0].UUID().String())
	chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{characteristicUUIDwavePlusData})
	if err != nil {
		return err
	}
	if len(chars) < 1 {
		return errors.New("empty characteristic list discovered")
	}

	char := chars[0]

	// BT characteristics are 512 bytes or less
	data := make([]byte, 512)
	len, err := char.Read(data)
	if err != nil {
		return err
	}

	buf := bytes.NewReader(data[0:len])

	parsedData := &wavePlusPayload{}
	err = binary.Read(buf, binary.LittleEndian, parsedData)
	if err != nil {
		return err
	}

	return s.parse(parsedData)
}

func (s *Sensor) parse(value *wavePlusPayload) error {
	if value.Version != 1 {
		return errors.New("incorrect version detected")
	}

	s.Humidity = float32(value.Humidity) / 2.0
	s.Illuminance = float32(value.Illuminance) * 1.0

	if value.Radon1DayAvg > 16383 {
		s.RadonShortTermAvg = -1
		return errors.New("radon value outside bounds")
	} else {
		s.RadonShortTermAvg = float32(value.Radon1DayAvg)
	}
	if value.RadonLongTermAvg > 16383 {
		s.RadonLongTermAvg = -1
		return errors.New("radon value outside bounds")
	} else {
		s.RadonLongTermAvg = float32(value.RadonLongTermAvg)
	}

	s.Temperature = float32(value.Temp) / 100.0
	s.RelativeAtmosphericPressure = float32(value.RelAtmPressure) / 50.0
	s.CO2Level = float32(value.CO2Level) * 1.0
	s.VOCLevel = float32(value.VOCLevel) * 1.0

	return nil
}

type wavePlusPayload struct {
	Version          uint8
	Humidity         uint8
	Illuminance      uint8
	Pad1             uint8
	Radon1DayAvg     uint16
	RadonLongTermAvg uint16
	Temp             uint16
	RelAtmPressure   uint16
	CO2Level         uint16
	VOCLevel         uint16
	Pad3             uint16
	Pad4             uint16
}
