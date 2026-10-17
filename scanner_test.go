package airthings

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

// fakeAdapter mimics the BlueZ adapter's scan lifecycle: Scan blocks until StopScan is called, and
// never reports a result, i.e. the sensor being looked for is never heard.
type fakeAdapter struct {
	mu          sync.Mutex
	scanning    chan struct{}
	scanStarted chan struct{}
	scanErr     error
	// ignoreStops makes the first N StopScan calls no-ops, as a stop landing before the adapter has
	// registered the scan would be.
	ignoreStops int
	stopCalls   int
}

func newFakeAdapter() *fakeAdapter { return &fakeAdapter{scanStarted: make(chan struct{}, 1)} }

func (f *fakeAdapter) Scan(cb func(*bluetooth.Adapter, bluetooth.ScanResult)) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	f.mu.Lock()
	f.scanning = make(chan struct{})
	ch := f.scanning
	f.mu.Unlock()
	f.scanStarted <- struct{}{}
	<-ch
	return nil
}

func (f *fakeAdapter) StopScan() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	if f.ignoreStops > 0 {
		f.ignoreStops--
		return nil
	}
	if f.scanning == nil {
		return errors.New("not scanning")
	}
	close(f.scanning)
	f.scanning = nil
	return nil
}

func (f *fakeAdapter) Connect(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
	return bluetooth.Device{}, errors.New("unexpected connect: no sensor should have been found")
}

func findWithin(t *testing.T, s *Scanner, ctx context.Context, wait time.Duration) (Sensor, error) {
	t.Helper()
	type result struct {
		sensor Sensor
		err    error
	}
	done := make(chan result, 1)
	go func() {
		sensor, err := s.FindSensor(ctx, 42)
		done <- result{sensor, err}
	}()
	select {
	case r := <-done:
		return r.sensor, r.err
	case <-time.After(wait):
		t.Fatal("FindSensor did not return; an unheard sensor must not block past ctx")
		return nil, nil
	}
}

func TestFindSensor_ReturnsWhenContextEndsWithoutAMatch(t *testing.T) {
	a := newFakeAdapter()
	s := &Scanner{adapter: a}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sensor, err := findWithin(t, s, ctx, 2*time.Second)
	if sensor != nil {
		t.Errorf("want no sensor, got %v", sensor)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want context.DeadlineExceeded, got %v", err)
	}
}

func TestFindSensor_CancellationStopsAScanTheFirstStopMissed(t *testing.T) {
	a := newFakeAdapter()
	a.ignoreStops = 3
	s := &Scanner{adapter: a}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-a.scanStarted
		cancel()
	}()

	_, err := findWithin(t, s, ctx, 2*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopCalls < 4 {
		t.Errorf("want StopScan retried past the ignored calls, got %d calls", a.stopCalls)
	}
}

func TestFindSensor_AlreadyCancelledDoesNotScan(t *testing.T) {
	a := newFakeAdapter()
	s := &Scanner{adapter: a}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := findWithin(t, s, ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
	select {
	case <-a.scanStarted:
		t.Error("a scan was started for an already-cancelled context")
	default:
	}
}

func TestFindSensor_ScanEndingWithoutAMatchIsSensorNotFound(t *testing.T) {
	a := newFakeAdapter()
	s := &Scanner{adapter: a}

	// Something other than ctx stops the scan.
	go func() {
		<-a.scanStarted
		_ = a.StopScan()
	}()

	_, err := findWithin(t, s, context.Background(), 2*time.Second)
	if !errors.Is(err, ErrSensorNotFound) {
		t.Errorf("want ErrSensorNotFound, got %v", err)
	}
}

func TestFindSensor_ScanErrorIsReturned(t *testing.T) {
	a := newFakeAdapter()
	a.scanErr = errors.New("adapter not powered")
	s := &Scanner{adapter: a}

	_, err := findWithin(t, s, context.Background(), time.Second)
	if err == nil || err.Error() != "adapter not powered" {
		t.Errorf("want the scan error, got %v", err)
	}
}

func TestFindSensor_NilAdapter(t *testing.T) {
	_, err := NewScanner(nil).FindSensor(context.Background(), 42)
	if !errors.Is(err, ErrInvalidBluetoothAdapter) {
		t.Errorf("want ErrInvalidBluetoothAdapter, got %v", err)
	}
}
