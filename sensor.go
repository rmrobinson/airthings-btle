package airthings

import (
	"context"
	"time"
)

// DeviceProfile contains the list of Bluetooth service IDs and the characteristic IDs on each service.
type DeviceProfile struct {
	ServiceUUID          string
	CharacteristicsUUIDs []string
}

// Measurement holds the most recent sensor readings produced by an Airthings
// device.  This struct is public so callers can save or display collections of
// measurements independently of the Bluetooth connection.
type Measurement struct {
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

// HistoryMeasurement holds a history hour block with 12 samples collected
// during a single hour. The radon values (short and long term) apply to the
// entire hour block, while the other fields are arrays of 12 per-sample
// measurements (e.g., one per 5 minutes within the hour). The Timestamp
// indicates when the hour block started.
type HistoryMeasurement struct {
	// Timestamp is when this hour block began (time.Time).
	Timestamp time.Time
	// RadonShortTermAvg represents the short-term average radon, in Bq/m3
	RadonShortTermAvg float32
	// RadonLongTermAvg represents the long-term average radon, in Bq/m3
	RadonLongTermAvg float32
	// Humidity holds 12 samples of humidity measurements (% relative humidity)
	Humidity [12]float32
	// Illuminance holds 12 samples of light level measurements (%)
	Illuminance [12]float32
	// Temperature holds 12 samples of temperature measurements (°C)
	Temperature [12]float32
	// RelativeAtmosphericPressure holds 12 samples of pressure (hPa)
	RelativeAtmosphericPressure [12]float32
	// CO2Level holds 12 samples of CO2 measurements (ppm)
	CO2Level [12]float32
	// VOCLevel holds 12 samples of VOC measurements (ppb)
	VOCLevel [12]float32
}

// Sensor represents an instance of an Airthings sensor
type Sensor interface {
	SerialNumber() int
	Address() string
	BatteryLevel() float32
	RSSI() int
	CurrentMeasurement() Measurement
	HistoricalMeasurements() []HistoryMeasurement

	Disconnect()
	Refresh(ctx context.Context) error
	RefreshHistory(ctx context.Context, hours int) error
	GetDeviceProfile() ([]DeviceProfile, error)
}
