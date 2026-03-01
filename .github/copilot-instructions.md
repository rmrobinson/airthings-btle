# Copilot instructions for airthings-btle

This repository is a small Go library and CLI for reading Airthings Wave Plus sensors over Bluetooth LE.

Key points for code edits
- Big picture: `scanner.go` discovers devices by scanning ManufacturerData (company ID `0x0334`) and matches a serial number; `sensor.go` connects to the device and reads a Wave Plus measurement characteristic (service UUID `b42e1c08-ade7-11e4-89d3-123b93f75cba`, characteristic `b42e2a68-ade7-11e4-89d3-123b93f75cba`). Follow the Scan -> ParseSerialNumber -> Connect -> Refresh -> parseCurrentValuesPayload flow.
- Data parsing: `sensor.go` reads the characteristic into a `currentValuesPayload` and uses `binary.LittleEndian` to decode. Many fields require scaling: Humidity (/2), Temperature (/100), Pressure (/50). Preserve those scaling factors when changing parsing logic.

	In addition to the main payload there are a handful of auxiliary "query" commands (query1, query2, query3, and history blocks). Each has its own private payload struct (`query1Payload`, `query2Payload`, etc.) and a helper named `parse<Query>X>Payload` that unpacks the raw bytes and performs any simple validation. The `query2Payload` now also exposes a `Battery` field (volts) derived from a byte within the raw data; not all responses include a valid reading. The tests in `sensor_test.go` show how to exercise these parsers and verify field values.

	Note: the current implementation only decodes the Wave Plus sensor payload (see `currentValuesPayload` and `parseCurrentValuesPayload` in `sensor.go`). Airthings manufactures multiple device models (for example: Wave Plus, Wave Radon, Wave Mini). Future work should add model-specific payload structs and parsers rather than changing the existing `wavePlusPayload` layout. Recommended approach when adding new models:

	- Add a new payload struct (e.g. `waveRadonPayload`) and a corresponding parser (`parseWaveRadonPayload`).
	- Detect the device model either by service/characteristic UUIDs discovered via `GetDeviceProfile()` or by a model/version field in the payload if available.
	- Keep parsing logic and scaling factors isolated per-model and add unit tests similar to `sensor_test.go` to validate decoding and scaling.
- Serial parsing: `utils.go`'s `ParseSerialNumber` expects exactly 6 bytes and builds the serial from the first 4 bytes. Keep this interface if adding new discovery logic.

- New data model: measurement fields have been extracted into a public `Measurement` struct. `Sensor` now holds a `CurrentMeasurement` and a slice `HistoricalMeasurements`. `parseCurrentValuesPayload` returns a `Measurement` and `Refresh()` assigns it to the sensor.

Build / test / run
- Build CLI: `go build -o airthings-cli ./cmd` (or `go build ./...` for library + tools).
- Run CLI (example): `./airthings-cli -serial 123456 -action get` or `go run ./cmd -serial 123456`.
- Tests: run `go test ./...`. See `sensor_test.go` and `utils_test.go` for examples of expected behavior.

Project conventions and patterns
- Use tinygo Bluetooth APIs: the project imports `tinygo.org/x/bluetooth`; code uses `bluetooth.DefaultAdapter` and `bluetooth.NewAdapter(adapterName)`. Keep adapter enablement checks and error handling consistent with `cmd/main.go`.
- Sentinel values: sensor numeric fields are initialized to `-1` to indicate unavailable values — preserve this convention when adding new fields or returning errors.
- Discovery vs read modes: `cmd/main.go` supports `-action get` and `-action discover`. New CLI flags should follow this simple flag-based approach.
- Avoid changing binary layout: the `currentValuesPayload` struct maps directly to the BLE payload order. If you need to extend parsing, add separate helper functions and unit tests rather than changing the struct order.

Integration and external deps
- Bluetooth stack: behavior depends on platform adapter support in `tinygo.org/x/bluetooth`. Tests run locally but real device integration requires a working Bluetooth adapter and appropriate OS permissions.
- go.mod uses `go 1.23.0` and `tinygo.org/x/bluetooth v0.14.0`. When updating dependencies, run `go mod tidy` and re-run tests.

Editing guidance and examples
- To add a new metric: add fields to `currentValuesPayload` only if they are actually present in the characteristic (confirm with `GetDeviceProfile()` / `discover` action). Add scaling in `parseWavePlusPayload` and unit tests in `sensor_test.go`.
- To add a discovery feature: extend `scanner.go`'s scan callback and use `ParseSerialNumber` for matching. Keep scan stop behavior (`s.adapter.StopScan()`) on match.
- Use Go best practices and standards, such as using Context for timeout and cancelation in long-running operations (e.g. scanning, connecting), and returning errors rather than panicking.

Files to reference while coding
- `sensor.go` — payload parsing, `Sensor` struct, `Refresh()` and `GetDeviceProfile()`.
- `scanner.go` — scanning and connection flow.
- `utils.go` — `ParseSerialNumber()` and `airthingsSerialNumberCompanyID`.
- `cmd/main.go` — CLI, flags, and example usage.

If anything in these instructions is unclear or you'd like me to emphasize other parts of the codebase (tests, CI, or packaging), tell me what to expand and I'll iterate.
