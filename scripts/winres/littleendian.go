package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

// littleEndianWriter appends little-endian fields to data. A value too big for its field is recorded in err instead of
// being truncated, so a run of appends needs only one error check at the end.
type littleEndianWriter struct {
	data []byte
	err  error
}

func (writer *littleEndianWriter) appendUint16(value uint16) {
	writer.data = binary.LittleEndian.AppendUint16(writer.data, value)
}

func (writer *littleEndianWriter) appendUint32(value uint32) {
	writer.data = binary.LittleEndian.AppendUint32(writer.data, value)
}

func (writer *littleEndianWriter) appendBytes(data []byte) {
	writer.data = append(writer.data, data...)
}

// appendIntAsUint16 appends value as a 16-bit field, or records an error naming field if it doesn't fit.
func (writer *littleEndianWriter) appendIntAsUint16(value int, field string) {
	converted, err := toUint16(value, field)
	writer.recordError(err)
	writer.appendUint16(converted)
}

// appendIntAsUint32 appends value as a 32-bit field, or records an error naming field if it doesn't fit.
func (writer *littleEndianWriter) appendIntAsUint32(value int, field string) {
	converted, err := toUint32(value, field)
	writer.recordError(err)
	writer.appendUint32(converted)
}

// pad appends zeros until the data's length is a multiple of alignment.
func (writer *littleEndianWriter) pad(alignment int) {
	writer.data = pad(writer.data, alignment)
}

// recordError keeps the first error, so the message names the first field that didn't fit.
func (writer *littleEndianWriter) recordError(err error) {
	if writer.err == nil {
		writer.err = err
	}
}

// pad returns data with zeros appended until its length is a multiple of alignment.
func pad(data []byte, alignment int) []byte {
	for len(data)%alignment != 0 {
		data = append(data, 0)
	}
	return data
}

// alignUp returns size rounded up to a multiple of alignment.
func alignUp(size, alignment int) int {
	return (size + alignment - 1) / alignment * alignment
}

// toUint16 returns value as a uint16, or an error naming field if it doesn't fit.
func toUint16(value int, field string) (uint16, error) {
	if value < 0 || value > math.MaxUint16 {
		return 0, fmt.Errorf("%s is %d, which doesn't fit in 16 bits", field, value)
	}
	return uint16(value), nil
}

// toUint32 returns value as a uint32, or an error naming field if it doesn't fit.
func toUint32(value int, field string) (uint32, error) {
	if value < 0 || int64(value) > math.MaxUint32 {
		return 0, fmt.Errorf("%s is %d, which doesn't fit in 32 bits", field, value)
	}
	return uint32(value), nil
}
