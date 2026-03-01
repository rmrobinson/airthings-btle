# Airthings Bluetooth Sensor

This library enables programs to read information provided by Airthings sensors over the Bluetooth Low Energy protocol. The protocol Airthings uses doesn't appear to be published anywhere, so it uses [a sample Python script](https://github.com/Airthings/waveplus-reader/blob/master/read_waveplus.py) provided by the Airthings team to understand how to parse the sensor data payload.

This has currently only been tested with the Wave Plus sensor.

This was extended to read the history records based on the work Simon Funk documented [here](https://sifter.org/~simon/journal/20191210.1.html) to allow for more battery-concious usage. It also leverages the work done in the custom Airthings Home Assistant component [here](https://github.com/sverrham/sensor.airthings_wave/commit/b7a35b00513a28e613103789390002fb0c7bf23f) to read the battery level from the sensor.
