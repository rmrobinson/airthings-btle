package airthings

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log"
	"time"

	"tinygo.org/x/bluetooth"
)

// The Bluetooth service UUID containing the Wave Plus measurement data.
var serviceUUIDWavePlusData, _ = bluetooth.ParseUUID("b42e1c08-ade7-11e4-89d3-123b93f75cba")

// The Bluetooth characteristic UUID for Wave Plus current measurement data.
var characteristicUUIDWavePlusCurrentData, _ = bluetooth.ParseUUID("b42e2a68-ade7-11e4-89d3-123b93f75cba")

// The Bluetooth Access Control Point characteristic is used for sending commands to the sensor and receiving responses.
var characteristicUUIDWavePlusCommand, _ = bluetooth.ParseUUID("b42e2d06-ade7-11e4-89d3-123b93f75cba")

// The Bluetooth Sensor Record characteristic is used to receive history blocks as out-of-band notifications.
var characteristicUUIDWavePlusSensorRecord, _ = bluetooth.ParseUUID("b42e2fc2-ade7-11e4-89d3-123b93f75cba")

type WavePlusSensor struct {
	device bluetooth.Device

	// serialNumber is the serial number of this sensor
	serialNumber int

	// rssi is the signal strength of the connection to this sensor. It is populated once, after scan.
	rssi int

	// batteryLevel contains the level of the battery of the sensor
	batteryLevel float32

	// currentMeasurement holds the most recent readings returned by Refresh()
	currentMeasurement Measurement

	// historicalMeasurements is a slice of past readings (hourly history blocks).
	historicalMeasurements []HistoryMeasurement
}

// newWavePlusSensor creates a Wave Plus sensor from a connected Bluetooth device which will be read from to retrieve values periodically.
func newWavePlusSensor(serialNumber int, device bluetooth.Device, rssi int) *WavePlusSensor {
	return &WavePlusSensor{
		device:       device,
		serialNumber: serialNumber,
		batteryLevel: -1,
		rssi:         rssi,
		currentMeasurement: Measurement{
			Humidity:                    -1,
			Illuminance:                 -1,
			RadonShortTermAvg:           -1,
			RadonLongTermAvg:            -1,
			Temperature:                 -1,
			RelativeAtmosphericPressure: -1,
			CO2Level:                    -1,
			VOCLevel:                    -1,
		},
	}
}

// SerialNumber returns the serial number of this sensor.
func (s *WavePlusSensor) SerialNumber() int {
	return s.serialNumber
}

// Address returns the Bluetooth address of this sensor.
func (s *WavePlusSensor) Address() string {
	return s.device.Address.String()
}

// RSSI returns the signal strength of the connection to this sensor. It is populated once, after scan.
func (s *WavePlusSensor) RSSI() int {
	return s.rssi
}

// BatteryLevel returns the battery level of the sensor, in percent. It is populated by Refresh().
func (s *WavePlusSensor) BatteryLevel() float32 {
	return s.batteryLevel
}

// CurrentMeasurement returns the most recent sensor readings retrieved by Refresh. It is a struct of human-readable values.
func (s *WavePlusSensor) CurrentMeasurement() Measurement {
	return s.currentMeasurement
}

// HistoricalMeasurements returns the historical measurements retrieved by RefreshHistory. It is a slice of past readings (hourly history blocks).
func (s *WavePlusSensor) HistoricalMeasurements() []HistoryMeasurement {
	return s.historicalMeasurements
}

// Disconnect disconnects from the sensor.
func (s *WavePlusSensor) Disconnect() {
	s.device.Disconnect()
}

// Refresh accesses the Bluetooth device to get the most recent readings for this device.
func (s *WavePlusSensor) Refresh(ctx context.Context) error {
	svcs, err := s.device.DiscoverServices([]bluetooth.UUID{serviceUUIDWavePlusData})
	if err != nil {
		return err
	} else if len(svcs) < 1 {
		return errors.New("empty service list discovered")
	}

	if err := s.refreshCurrentData(ctx, &svcs[0]); err != nil {
		return err
	}
	if err := s.query2(ctx, &svcs[0]); err != nil {
		return err
	}
	return nil
}

// RefreshHistory retrieves historical measurements for this sensor. It takes in the number of hours back to query.
func (s *WavePlusSensor) RefreshHistory(ctx context.Context, hours int) error {
	svcs, err := s.device.DiscoverServices([]bluetooth.UUID{serviceUUIDWavePlusData})
	if err != nil {
		return err
	} else if len(svcs) < 1 {
		return errors.New("empty service list discovered")
	}

	if err := s.getHistory(ctx, &svcs[0], hours); err != nil {
		return err
	}
	return nil
}

// refreshCurrentData reads the current values characteristic from the given service,
// parses the payload, and updates the sensor's CurrentMeasurement.
func (s *WavePlusSensor) refreshCurrentData(_ context.Context, svc *bluetooth.DeviceService) error {
	chars, err := svc.DiscoverCharacteristics([]bluetooth.UUID{characteristicUUIDWavePlusCurrentData})
	if err != nil {
		return err
	} else if len(chars) < 1 {
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

	parsedData := &currentValuesPayload{}
	if err := binary.Read(buf, binary.LittleEndian, parsedData); err != nil {
		return err
	}

	m, err := parseCurrentValuesPayload(parsedData)
	if err != nil {
		return err
	}
	s.currentMeasurement = m
	return nil
}

// query2 executes a query2 command to retrieve elapsed time, ambient light, and battery
// information. It writes the 0x6d command to the command characteristic and reads the
// response from the sensor record characteristic.
func (s *WavePlusSensor) query2(ctx context.Context, svc *bluetooth.DeviceService) error {
	// Discover the command characteristic.
	cmdChars, err := svc.DiscoverCharacteristics([]bluetooth.UUID{characteristicUUIDWavePlusCommand})
	if err != nil {
		return err
	} else if len(cmdChars) < 1 {
		return errors.New("command characteristic not found")
	}

	// Write the query2 command (0x6d) to the command characteristic and
	// enable notifications on that same characteristic. The device will
	// respond with a notification containing the 28-byte payload.
	char := cmdChars[0]
	respChan := make(chan []byte, 1)
	if err := char.EnableNotifications(func(buf []byte) {
		// copy buffer since tinygo may reuse slice
		dataCopy := make([]byte, len(buf))
		copy(dataCopy, buf)
		select {
		case respChan <- dataCopy:
		default:
		}
	}); err != nil {
		return err
	}
	// ensure notifications disabled when we leave or if context ends
	defer char.EnableNotifications(nil)

	cmdData := []byte{0x6d}
	if _, err := char.WriteWithoutResponse(cmdData); err != nil {
		return err
	}

	var respData []byte
	select {
	case respData = <-respChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	if respData[0] != cmdData[0] {
		return errors.New("unexpected query2 response command byte")
	}
	respData = respData[2:] // strip off command bytes
	if len(respData) != 28 {
		return errors.New("unexpected query2 response length")
	}

	// Parse the response into a query2Payload and update battery.
	q2 := &query2Payload{Raw: respData}
	if err := parseQuery2Payload(q2); err != nil {
		return err
	}
	// convert voltage (volts) into 0–100% using V_MIN/V_MAX
	const (
		V_MIN = 2.0 // volts
		V_MAX = 3.0
	)
	pct := int((q2.Voltage - V_MIN) / (V_MAX - V_MIN) * 100.0)
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	s.batteryLevel = float32(pct)
	return nil
}

// getHistory executes a command to retrieve historical data for this sensor. It takes in the number of hours back to query.
func (s *WavePlusSensor) getHistory(ctx context.Context, svc *bluetooth.DeviceService, hours int) error {
	// Discover the command characteristic.
	cmdChars, err := svc.DiscoverCharacteristics([]bluetooth.UUID{characteristicUUIDWavePlusCommand})
	if err != nil {
		return err
	} else if len(cmdChars) < 1 {
		return errors.New("command characteristic not found")
	}

	sensorChars, err := svc.DiscoverCharacteristics([]bluetooth.UUID{characteristicUUIDWavePlusSensorRecord})
	if err != nil {
		return err
	} else if len(sensorChars) < 1 {
		return errors.New("sensor record characteristic not found")
	}

	// Write the getHistory command (0x6d) to the command characteristic and
	// enable notifications on that same characteristic. The device will
	// respond with a notification containing the 28-byte payload.
	cmdChar := cmdChars[0]
	cmdRespChan := make(chan []byte, 1)
	if err := cmdChar.EnableNotifications(func(buf []byte) {
		// copy buffer since tinygo may reuse slice
		dataCopy := make([]byte, len(buf))
		copy(dataCopy, buf)
		select {
		case cmdRespChan <- dataCopy:
		default:
		}
	}); err != nil {
		return err
	}
	// ensure notifications disabled when we leave or if context ends
	defer cmdChar.EnableNotifications(nil)

	sensorChar := sensorChars[0]
	sensorRespChan := make(chan []byte, hours)
	if err := sensorChar.EnableNotifications(func(buf []byte) {
		// copy buffer since tinygo may reuse slice
		dataCopy := make([]byte, len(buf))
		copy(dataCopy, buf)
		select {
		case sensorRespChan <- dataCopy:
		default:
		}
	}); err != nil {
		return err
	}
	// ensure notifications disabled when we leave or if context ends
	defer sensorChar.EnableNotifications(nil)

	getHistoryCmd := &getHistoryCommand{
		CommandID:      0x01,
		Field1:         2,
		Field2:         0,
		HoursToInclude: uint16(hours),
		Field4:         0,
	}
	getHistoryCmdBuf := &bytes.Buffer{}
	if err := binary.Write(getHistoryCmdBuf, binary.LittleEndian, getHistoryCmd); err != nil {
		return err
	}

	cmdData := getHistoryCmdBuf.Bytes()
	_, err = cmdChar.WriteWithoutResponse(cmdData)
	if err != nil {
		return err
	}

	var cmdResp []byte
	select {
	case cmdResp = <-cmdRespChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	if cmdResp[0] != cmdData[0] {
		return errors.New("unexpected getHistory command response command byte")
	}
	cmdResp = cmdResp[2:] // strip off command bytes
	if len(cmdResp) != 4 {
		return errors.New("unexpected getHistory command response length")
	}

	cmdRespHours := binary.LittleEndian.Uint32(cmdResp)
	if cmdRespHours-1 != uint32(hours) {
		// Unclear why but the command seems to return 1 more than we request every time. Alert if it's different than this expectation.
		log.Printf("requested %d hours of history, sensor responded with %d hours available\n", hours, cmdRespHours)
	}

	count := hours
	for {
		select {
		case sensorResp := <-sensorRespChan:
			buf := bytes.NewReader(sensorResp)

			parsedData := &historyHourPayload{}
			if err := binary.Read(buf, binary.LittleEndian, parsedData); err != nil {
				return err
			}

			historyRecord, err := parseHistoryHourPayload(parsedData)
			if err != nil {
				return err
			}

			s.historicalMeasurements = append(s.historicalMeasurements, historyRecord)

			if count--; count <= 0 {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetDeviceProfile iterates the list of Bluetooth services available and retrieves all their available characteristic UUIDs
func (s *WavePlusSensor) GetDeviceProfile() ([]DeviceProfile, error) {
	svcs, err := s.device.DiscoverServices(nil)
	if err != nil {
		return nil, err
	} else if len(svcs) < 1 {
		return nil, errors.New("empty service list discovered")
	}

	ret := []DeviceProfile{}
	for _, svc := range svcs {
		svcRet := DeviceProfile{
			ServiceUUID: svc.UUID().String(),
		}

		chars, err := svc.DiscoverCharacteristics(nil)
		if err != nil {
			return nil, err
		}

		if len(chars) < 1 {
			ret = append(ret, svcRet)
			continue
		}

		for _, char := range chars {
			svcRet.CharacteristicsUUIDs = append(svcRet.CharacteristicsUUIDs, char.UUID().String())
		}

		ret = append(ret, svcRet)
	}

	return ret, nil
}

// currentValuesPayload contains the current values reported by the sensor, in the binary format read from the Bluetooth characteristic.
type currentValuesPayload struct {
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

// query1Payload holds the raw response from a query1 command. The response
// contains a header (3 bytes: one signed int8 and two uint8s) followed by
// nine uint16 fields. These fields' meanings are not fully documented and are
// parsed for completeness but rarely used by callers.
type query1Payload struct {
	Raw  []byte
	a    int8
	b    uint8
	c    uint8
	vals [9]uint16
}

// query2Payload holds decoded fields from a query2 response. The raw response
// is 28 bytes: a 4-byte little-endian elapsed time, 24 bytes of data including
// ambient light at offset 6 and a battery millivolt reading at offset 21.
type query2Payload struct {
	Raw          []byte
	a            uint32
	b            [12]uint8
	c            [6]uint16
	TimeElapsed  uint32
	AmbientLight uint8
	Voltage      float32
}

// query3Payload holds decoded fields from a query3 response. The raw response
// is 13 bytes containing a command echo (1 byte), series start timestamp
// (4 bytes), and record count (2 bytes, at offset 9), plus other fields.
type query3Payload struct {
	SeriesStart uint32
	NumRecords  uint16
	Raw         []byte
}

// getHistoryCommand holds fields to be encoded to make a getHistory request.
// CommandID should be 0x01; Field 1 should be 2, Field 2 and Field 4 should be 0
type getHistoryCommand struct {
	CommandID      uint8
	Field1         uint16
	Field2         uint16
	HoursToInclude uint16
	Field4         uint16
}

// historyHourPayload corresponds to one 230-byte history block returned by
// the sensor. It holds raw sensor readings (temperature, humidity, pressure,
// CO2, VOC, ambient light, etc.) from 12 one-hour periods plus metadata.
// Fields use raw units; scaling to human-readable values happens in the
// returned Measurement array from parseHistoryHourPayload.
type historyHourPayload struct {
	// The history hour block is 230 bytes and contains all measurements
	// collected during a single hour. The layout (from the Python
	// implementation) is:
	//  - 8 uint16 unused header values
	//  - 2 uint16 radon values
	//  - 12 uint16 temperature samples
	//  - 12 uint8  humidity samples
	//  - 12 uint16 pressure samples
	//  - 12 uint16 CO2 samples
	//  - 12 entries of (uint16 x4, uint16 voc, uint32 x3)  (interleaved)
	//  - final block: 12 uint8 ambient light, 6 uint8 unused, uint32 series_start,
	//    3 uint16 unused, uint16 recno
	UnusedHeader   [8]uint16
	Radon          [2]uint16
	TemperatureRaw [12]uint16 // values which must be scaled (t-27315)/100
	HumidityRaw    [12]uint8  // each is raw humidity*2
	PressureRaw    [12]uint16 // raw pressure*50
	CO2Raw         [12]uint16

	// Interleaved per-sample entries (x4, voc, x3) repeated 12 times.
	Entries [12]historyEntry

	AmbientLight [12]uint8
	UnusedTail1  [6]uint8
	SeriesStart  uint32
	UnusedTail2  [3]uint16
	RecNo        uint16
}

// historyEntry is a single interleaved measurement in the hour block: two
// uint16 values followed by one uint32 (matches '<2HL' from Python parsing).
type historyEntry struct {
	X4  uint16
	VOC uint16
	X3  uint32
}

// parseCurrentValuesPayload decodes the raw current values payload into a Measurement struct with human-readable values.
func parseCurrentValuesPayload(value *currentValuesPayload) (Measurement, error) {
	var m Measurement
	if value.Version != 1 {
		return m, errors.New("incorrect version detected")
	}

	m.Humidity = float32(value.Humidity) / 2.0
	m.Illuminance = float32(value.Illuminance) * 1.0

	if value.Radon1DayAvg > 16383 {
		// leave default -1 and return error
		return m, errors.New("radon value outside bounds")
	}
	m.RadonShortTermAvg = float32(value.Radon1DayAvg)

	if value.RadonLongTermAvg > 16383 {
		return m, errors.New("radon value outside bounds")
	}
	m.RadonLongTermAvg = float32(value.RadonLongTermAvg)

	m.Temperature = float32(value.Temp) / 100.0
	m.RelativeAtmosphericPressure = float32(value.RelAtmPressure) / 50.0
	m.CO2Level = float32(value.CO2Level) * 1.0
	m.VOCLevel = float32(value.VOCLevel) * 1.0

	return m, nil
}

// parseQuery1Payload decodes the raw bytes held in v.Raw into the other fields.
func parseQuery1Payload(v *query1Payload) error {
	if len(v.Raw) != 21 {
		return errors.New("query1 payload length")
	}
	v.a = int8(v.Raw[0])
	v.b = v.Raw[1]
	v.c = v.Raw[2]
	for i := 0; i < 9; i++ {
		off := 3 + i*2
		v.vals[i] = binary.LittleEndian.Uint16(v.Raw[off : off+2])
	}
	return nil
}

// parseQuery2Payload extracts TimeElapsed and AmbientLight from the raw data.
func parseQuery2Payload(v *query2Payload) error {
	if len(v.Raw) != 28 {
		return errors.New("query2 payload length")
	}
	v.TimeElapsed = binary.LittleEndian.Uint32(v.Raw[0:4])
	v.AmbientLight = v.Raw[5]
	v.Voltage = float32(binary.LittleEndian.Uint16(v.Raw[24:26])) / 1000.0
	return nil
}

// parseQuery3Payload fills v.SeriesStart and NumRecords from the raw bytes.
func parseQuery3Payload(v *query3Payload) error {
	if len(v.Raw) != 13 {
		return errors.New("query3 payload length")
	}
	v.SeriesStart = binary.LittleEndian.Uint32(v.Raw[1:5])
	// three 16-bit values follow the initial 5 bytes; the third of those is
	// num_records (index 4 in the python unpack result), which lives at
	// raw[9:11].
	v.NumRecords = binary.LittleEndian.Uint16(v.Raw[9:11])
	return nil
}

// parseHistoryHourPayload decodes a 230-byte history block into a HistoryMeasurement.
// The payload contains 12 sub-samples collected during a single hour, with the radon
// values applying to the entire hour and the other fields present as arrays of 12
// per-sample measurements. The timestamp is calculated from SeriesStart and RecNo.
func parseHistoryHourPayload(v *historyHourPayload) (HistoryMeasurement, error) {
	// Calculate the timestamp for this hour block using RecNo (which represents
	// the hour offset from SeriesStart).
	timestamp := time.Unix(int64(v.SeriesStart+uint32(v.RecNo)*3600), 0)

	hm := HistoryMeasurement{
		Timestamp:         timestamp,
		RadonShortTermAvg: float32(v.Radon[0]),
		RadonLongTermAvg:  float32(v.Radon[1]),
	}

	// Populate the 12 samples for each per-sample field.
	for i := 0; i < 12; i++ {
		hm.Temperature[i] = float32(int(v.TemperatureRaw[i])-27315) / 100.0
		hm.Humidity[i] = float32(v.HumidityRaw[i]) / 2.0
		hm.RelativeAtmosphericPressure[i] = float32(v.PressureRaw[i]) / 50.0
		hm.CO2Level[i] = float32(v.CO2Raw[i])
		hm.VOCLevel[i] = float32(v.Entries[i].VOC)
		hm.Illuminance[i] = float32(v.AmbientLight[i])
	}

	return hm, nil
}
