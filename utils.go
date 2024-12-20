package airthings

import (
	"errors"
)

const airthingsSerialNumberCompanyID = 0x0334

func ParseSerialNumber(input []byte) (int, error) {
	if len(input) != 6 {
		return -1, errors.New("invalid serial number length")
	}

	sn := int(input[0])
	sn |= int(input[1]) << 8
	sn |= int(input[2]) << 16
	sn |= int(input[3]) << 24

	return sn, nil
}
