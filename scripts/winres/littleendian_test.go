package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestToUint16(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		wantValue uint16
		wantError bool
	}{
		{"zero fits", 0, 0, false},
		{"largest value fits", math.MaxUint16, math.MaxUint16, false},
		{"one past the largest returns an error", math.MaxUint16 + 1, 0, true},
		{"negative returns an error", -1, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := toUint16(test.value, "test field")
			if (err != nil) != test.wantError || got != test.wantValue {
				t.Errorf("toUint16(%d) = %d, %v; want %d with error %v", test.value, got, err, test.wantValue,
					test.wantError)
			}
		})
	}
}

func TestToUint32(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		wantValue uint32
		wantError bool
	}{
		{"zero fits", 0, 0, false},
		{"largest value fits", math.MaxUint32, math.MaxUint32, false},
		{"one past the largest returns an error", math.MaxUint32 + 1, 0, true},
		{"negative returns an error", -1, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := toUint32(test.value, "test field")
			if (err != nil) != test.wantError || got != test.wantValue {
				t.Errorf("toUint32(%d) = %d, %v; want %d with error %v", test.value, got, err, test.wantValue,
					test.wantError)
			}
		})
	}
}

func TestLittleEndianWriter_KeepsFirstError(t *testing.T) {
	var writer littleEndianWriter
	writer.appendIntAsUint16(0x0102, "first field")
	writer.appendIntAsUint16(math.MaxUint16+1, "second field")
	writer.appendIntAsUint32(-1, "third field")
	writer.pad(8)
	if writer.err == nil || !strings.Contains(writer.err.Error(), "second field is 65536") {
		t.Errorf("error = %v, want the second field's", writer.err)
	}
	// A field that doesn't fit is written as zeros, so the layout of what follows stays the same.
	if want := []byte{2, 1, 0, 0, 0, 0, 0, 0}; !bytes.Equal(writer.data, want) {
		t.Errorf("data = % x, want % x", writer.data, want)
	}
}
