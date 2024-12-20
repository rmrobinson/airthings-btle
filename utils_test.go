package airthings

import "testing"

func TestParseSerialNumber(t *testing.T) {
	tests := map[string]struct {
		input []byte
		want  int
		err   error
	}{
		"base case": {input: []byte{21, 217, 166, 174, 9, 0}, want: 2930170133, err: nil},
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
