package main

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// The layout of an .ico file: an ICONDIR header, then one ICONDIRENTRY per image.
const (
	icoHeaderSize = 6
	icoEntrySize  = 16
	icoTypeIcon   = 1 // the ICONDIR type of an icon, as opposed to a cursor
	// Offsets within an ICONDIRENTRY of the image's size in bytes and its offset in the file.
	icoEntrySizeField   = 8
	icoEntryOffsetField = 12
)

// parseICO splits an .ico file into its images, as RT_ICON resources 1 to n, followed by the RT_GROUP_ICON resource 1
// that lists them.
func parseICO(file []byte) ([]resource, error) {
	if len(file) < icoHeaderSize {
		return nil, errors.New("not an .ico file")
	}
	reserved := binary.LittleEndian.Uint16(file[0:])
	imageType := binary.LittleEndian.Uint16(file[2:])
	if reserved != 0 || imageType != icoTypeIcon {
		return nil, errors.New("not an .ico file")
	}
	imageCount := binary.LittleEndian.Uint16(file[4:])
	if imageCount == 0 || len(file) < icoHeaderSize+icoEntrySize*int(imageCount) {
		return nil, errors.New(".ico file has no images or is cut short")
	}
	group := binary.LittleEndian.AppendUint16(nil, 0) // reserved
	group = binary.LittleEndian.AppendUint16(group, icoTypeIcon)
	group = binary.LittleEndian.AppendUint16(group, imageCount)
	resources := make([]resource, 0, int(imageCount)+1)
	for index := range imageCount {
		entryStart := icoHeaderSize + icoEntrySize*int(index)
		entry := file[entryStart : entryStart+icoEntrySize]
		imageSize := binary.LittleEndian.Uint32(entry[icoEntrySizeField:])
		imageStart := binary.LittleEndian.Uint32(entry[icoEntryOffsetField:])
		imageEnd := uint64(imageStart) + uint64(imageSize)
		if imageEnd > uint64(len(file)) {
			return nil, fmt.Errorf(".ico image %d is outside the file", index+1)
		}
		iconID := index + 1
		image := file[imageStart:imageEnd]
		resources = append(resources, resource{typeID: resourceTypeIcon, nameID: iconID, data: image})
		// GRPICONDIRENTRY: the ICONDIRENTRY's fields up to the file offset, then the resource ID in its place.
		group = append(group, entry[:icoEntryOffsetField]...)
		group = binary.LittleEndian.AppendUint16(group, iconID)
	}
	return append(resources, resource{typeID: resourceTypeGroupIcon, nameID: 1, data: group}), nil
}
