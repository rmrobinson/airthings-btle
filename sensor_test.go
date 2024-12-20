package airthings

import "testing"

func TestSensorParse(t *testing.T) {
	tests := map[string]struct {
		input []byte
		want  int
		err   error
	}{
		"base case": {input: []byte{186, 92, 247, 147, 59, 18, 211, 137, 228, 17, 231, 173, 104, 42, 46, 180}, want: 2930170133, err: nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseSerialNumber(tc.input)
			if err != tc.err {
				t.Fatalf("expected err: %v, got %v", tc.err, err)
			} else if got != tc.want {
				t.Fatalf("expected: %d, got %d", tc.want, got)
			}
		})
	}
}
