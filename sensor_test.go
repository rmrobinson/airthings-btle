package airthings

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestParseCurrentValuesPayload(t *testing.T) {
	// raw payload created using
	// struct.pack('<BBBBHHHHHHHH', 1,80,5,0,500,1000,2150,50650,400,50,0,0)
	raw := []byte{
		0x01, 0x50, 0x05, 0x00,
		0xf4, 0x01, 0xe8, 0x03,
		0x66, 0x08, 0xda, 0xc5,
		0x90, 0x01, 0x32, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
	}
	var cv currentValuesPayload
	if err := binary.Read(bytes.NewReader(raw), binary.LittleEndian, &cv); err != nil {
		t.Fatalf("binary.Read: %v", err)
	}
	m, err := parseCurrentValuesPayload(&cv)
	if err != nil {
		t.Fatalf("parseCurrentValuesPayload: %v", err)
	}
	if m.Humidity != 40.0 {
		t.Fatalf("expected humidity 40.0, got %v", m.Humidity)
	}
	if m.Illuminance != 5.0 {
		t.Fatalf("expected illuminance 5.0, got %v", m.Illuminance)
	}
	if m.RadonShortTermAvg != 500 {
		t.Fatalf("unexpected radon short: %v", m.RadonShortTermAvg)
	}
	if m.RadonLongTermAvg != 1000 {
		t.Fatalf("unexpected radon long: %v", m.RadonLongTermAvg)
	}
	if m.Temperature != 21.50 {
		t.Fatalf("unexpected temperature: %v", m.Temperature)
	}
	if m.RelativeAtmosphericPressure != float32(50650)/50.0 {
		t.Fatalf("unexpected pressure: %v", m.RelativeAtmosphericPressure)
	}
	if m.CO2Level != 400.0 {
		t.Fatalf("unexpected CO2: %v", m.CO2Level)
	}
	if m.VOCLevel != 50.0 {
		t.Fatalf("unexpected VOC: %v", m.VOCLevel)
	}
}
func TestParseQuery1Payload(t *testing.T) {
	raw := []byte{
		0x01, 0x00, 0x00, 0x12, 0x00, 0x3c, 0x46, 0x1e,
		0x19, 0x64, 0x00, 0x96, 0x00, 0xfa, 0x00, 0xd0,
		0x07, 0x20, 0x03, 0xe8, 0x03,
	}
	q := &query1Payload{Raw: raw}
	if err := parseQuery1Payload(q); err != nil {
		t.Fatalf("parseQuery1Payload: %v", err)
	}
	if q.a != 1 || q.b != 0 || q.c != 0 {
		t.Fatalf("unexpected header fields: %+v", q)
	}
	if q.vals[0] != 18 || q.vals[1] != 17980 || q.vals[2] != 6430 {
		t.Fatalf("unexpected values: %v", q.vals[:3])
	}
}

func TestParseQuery2Payload(t *testing.T) {
	raw := []byte{
		0x0a, 0x96, 0x0b, 0x00, 0x02, 0x26, 0x88, 0x41,
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x4b, 0x1c,
		0x00, 0xb8, 0x34, 0x1e, 0x64, 0xc5, 0x00, 0x0a, 0x0b,
		0x09, 0x00,
	}
	q := &query2Payload{Raw: raw}
	if err := parseQuery2Payload(q); err != nil {
		t.Fatalf("parseQuery2Payload: %v", err)
	}
	if q.TimeElapsed != 759306 {
		t.Fatalf("unexpected TimeElapsed %d", q.TimeElapsed)
	}
	if q.AmbientLight != 38 {
		t.Fatalf("unexpected AmbientLight %d", q.AmbientLight)
	}
	if q.Voltage != 2.826 {
		t.Fatalf("unexpected Voltage %v", q.Voltage)
	}
}

func TestParseQuery3Payload(t *testing.T) {
	raw := []byte{0x54, 0xa9, 0x8e, 0x0d, 0x5d, 0x00, 0x01, 0x29, 0xdf, 0x26, 0x0f, 0xff, 0xff}
	q := &query3Payload{Raw: raw}
	if err := parseQuery3Payload(q); err != nil {
		t.Fatalf("parseQuery3Payload: %v", err)
	}
	if q.SeriesStart != binary.LittleEndian.Uint32(raw[1:5]) {
		t.Fatalf("unexpected SeriesStart %d", q.SeriesStart)
	}
	if q.NumRecords != binary.LittleEndian.Uint16(raw[9:11]) {
		t.Fatalf("unexpected NumRecords %d", q.NumRecords)
	}
}

func TestHistoryHourPayloadLayout(t *testing.T) {
	var orig historyHourPayload
	orig.Radon = [2]uint16{1234, 4567}
	for i := 0; i < 12; i++ {
		orig.TemperatureRaw[i] = uint16(i)
		orig.HumidityRaw[i] = uint8(i + 1)
		orig.PressureRaw[i] = uint16(100 + i)
		orig.CO2Raw[i] = uint16(200 + i)
		orig.Entries[i].VOC = uint16(300 + i)
		orig.AmbientLight[i] = uint8(50 + i)
		orig.Entries[i].X4 = uint16(400 + i)
		orig.Entries[i].X3 = uint32(500 + i)
	}
	orig.SeriesStart = 0xdeadbeef
	orig.RecNo = 5 // must be < 12 to satisfy parseHistoryHourPayload

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, &orig); err != nil {
		t.Fatalf("binary.Write failed: %v", err)
	}
	var decoded historyHourPayload
	if err := binary.Read(bytes.NewReader(buf.Bytes()), binary.LittleEndian, &decoded); err != nil {
		t.Fatalf("binary.Read failed: %v", err)
	}
	if decoded.Radon[0] != 1234 || decoded.Radon[1] != 4567 {
		t.Fatalf("radon mismatch: %+v", decoded.Radon)
	}
	if decoded.TemperatureRaw[5] != 5 {
		t.Fatalf("temp mismatch: %v", decoded.TemperatureRaw[5])
	}
	hm, err := parseHistoryHourPayload(&decoded)
	if err != nil {
		t.Fatalf("parseHistoryHourPayload: %v", err)
	}
	// expected timestamp for RecNo (converted to time.Time)
	expectedUnix := int64(decoded.SeriesStart + uint32(decoded.RecNo)*3600)
	expectedTimestamp := time.Unix(expectedUnix, 0)
	if hm.Timestamp != expectedTimestamp {
		t.Fatalf("timestamp mismatch: got %v want %v", hm.Timestamp, expectedTimestamp)
	}
	if hm.RadonShortTermAvg != float32(decoded.Radon[0]) {
		t.Fatalf("radon short term mismatch: got %v want %v", hm.RadonShortTermAvg, float32(decoded.Radon[0]))
	}
	if hm.RadonLongTermAvg != float32(decoded.Radon[1]) {
		t.Fatalf("radon long term mismatch: got %v want %v", hm.RadonLongTermAvg, float32(decoded.Radon[1]))
	}
	// verify sample at index 5
	idx := 5
	expectedTemp := float32(int(decoded.TemperatureRaw[idx])-27315) / 100.0
	if hm.Temperature[idx] != expectedTemp {
		t.Fatalf("scaled temperature wrong at index %d: got %v want %v", idx, hm.Temperature[idx], expectedTemp)
	}
	expectedHum := float32(decoded.HumidityRaw[idx]) / 2.0
	if hm.Humidity[idx] != expectedHum {
		t.Fatalf("scaled humidity wrong at index %d: got %v want %v", idx, hm.Humidity[idx], expectedHum)
	}
	expectedPressure := float32(decoded.PressureRaw[idx]) / 50.0
	if hm.RelativeAtmosphericPressure[idx] != expectedPressure {
		t.Fatalf("scaled pressure wrong at index %d: got %v want %v", idx, hm.RelativeAtmosphericPressure[idx], expectedPressure)
	}
	// verify CO2, VOC, Illuminance at index 5
	if hm.CO2Level[idx] != float32(decoded.CO2Raw[idx]) {
		t.Fatalf("CO2 mismatch at index %d: got %v want %v", idx, hm.CO2Level[idx], float32(decoded.CO2Raw[idx]))
	}
	if hm.VOCLevel[idx] != float32(decoded.Entries[idx].VOC) {
		t.Fatalf("VOC mismatch at index %d: got %v want %v", idx, hm.VOCLevel[idx], float32(decoded.Entries[idx].VOC))
	}
	if hm.Illuminance[idx] != float32(decoded.AmbientLight[idx]) {
		t.Fatalf("illuminance mismatch at index %d: got %v want %v", idx, hm.Illuminance[idx], float32(decoded.AmbientLight[idx]))
	}
}
