package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// makeICO returns an .ico file with imageCount 16x16 images, all of them the one byte at the end of the file.
func makeICO(imageCount uint16) []byte {
	imageStart := icoHeaderSize + icoEntrySize*uint32(imageCount)
	file := binary.LittleEndian.AppendUint16(nil, 0)
	file = binary.LittleEndian.AppendUint16(file, icoTypeIcon)
	file = binary.LittleEndian.AppendUint16(file, imageCount)
	for range imageCount {
		file = append(file, 16, 16, 0, 0)                 // width, height, colors, reserved
		file = binary.LittleEndian.AppendUint16(file, 1)  // planes
		file = binary.LittleEndian.AppendUint16(file, 32) // bits per pixel
		file = binary.LittleEndian.AppendUint32(file, 1)  // image size
		file = binary.LittleEndian.AppendUint32(file, imageStart)
	}
	return append(file, 0xAB)
}

// withUint32 returns a copy of file with value written at offset.
func withUint32(file []byte, offset int, value uint32) []byte {
	changed := bytes.Clone(file)
	binary.LittleEndian.PutUint32(changed[offset:], value)
	return changed
}

func TestParseICO(t *testing.T) {
	twoImages := makeICO(2)
	resources, err := parseICO(twoImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 3 {
		t.Fatalf("got %d resources, want 2 icons and a group", len(resources))
	}
	for index, icon := range resources[:2] {
		if icon.typeID != resourceTypeIcon || int(icon.nameID) != index+1 || !bytes.Equal(icon.data, []byte{0xAB}) {
			t.Errorf("icon %d = %+v, want RT_ICON %d holding the one image byte", index, icon, index+1)
		}
	}
	group := resources[2]
	wantGroup := []byte{
		0, 0, 1, 0, 2, 0, // reserved, type, count
		16, 16, 0, 0, 1, 0, 32, 0, 1, 0, 0, 0, 1, 0, // first image, as resource 1
		16, 16, 0, 0, 1, 0, 32, 0, 1, 0, 0, 0, 2, 0, // second image, as resource 2
	}
	if group.typeID != resourceTypeGroupIcon || group.nameID != 1 || !bytes.Equal(group.data, wantGroup) {
		t.Errorf("group = %+v, want RT_GROUP_ICON 1 holding % x", group, wantGroup)
	}
}

func TestParseICO_RejectsBadInput(t *testing.T) {
	oneImage := makeICO(1)
	const firstEntry = icoHeaderSize
	tests := []struct {
		name      string
		file      []byte
		wantError string
	}{
		{"empty file", nil, "not an .ico file"},
		{"header cut short", oneImage[:5], "not an .ico file"},
		{"nonzero reserved field", append([]byte{1}, oneImage[1:]...), "not an .ico file"},
		{"cursor instead of an icon", append([]byte{0, 0, 2}, oneImage[3:]...), "not an .ico file"},
		{"no images", []byte{0, 0, 1, 0, 0, 0}, "no images or is cut short"},
		{"directory cut short", oneImage[:icoHeaderSize+icoEntrySize-1], "no images or is cut short"},
		{"image past the end", withUint32(oneImage, firstEntry+icoEntrySizeField, 2), "image 1 is outside the file"},
		{
			"image offset past the end",
			withUint32(oneImage, firstEntry+icoEntryOffsetField, math.MaxUint32),
			"image 1 is outside the file",
		},
	}
	for _, test := range tests {
		t.Run(test.name+" returns an error", func(t *testing.T) {
			_, err := parseICO(test.file)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("error = %v, want one containing %q", err, test.wantError)
			}
		})
	}
}

// FuzzParseICO checks that parseICO never panics, and that whatever it accepts has one icon per directory entry, a
// group listing them all, and can be built into a resource object. Seeds also live in testdata/fuzz/FuzzParseICO.
func FuzzParseICO(f *testing.F) {
	f.Add(readTestIcon(f))
	f.Add(makeICO(1))
	f.Add(makeICO(3)[:40])
	f.Add([]byte{0, 0, 1, 0, 0, 0})
	f.Fuzz(func(t *testing.T, file []byte) {
		resources, err := parseICO(file)
		if err != nil {
			return
		}
		imageCount := int(binary.LittleEndian.Uint16(file[4:]))
		checkParsedICO(t, resources, imageCount)
		// Up to 65533 images, the icons, group, and version info fit in a COFF section's 65535 relocations.
		if imageCount <= math.MaxUint16-2 {
			if _, err := buildResourceObject(file, versionDetails{version: "1.0.0"}); err != nil {
				t.Errorf("building a parsed icon: %v", err)
			}
		}
	})
}

// checkParsedICO checks that resources are imageCount icons numbered from 1, followed by a group listing them.
func checkParsedICO(t *testing.T, resources []resource, imageCount int) {
	t.Helper()
	if len(resources) != imageCount+1 {
		t.Fatalf("got %d resources for %d images", len(resources), imageCount)
	}
	for index, icon := range resources[:imageCount] {
		if icon.typeID != resourceTypeIcon || int(icon.nameID) != index+1 {
			t.Errorf("resource %d is type %d ID %d, want RT_ICON %d", index, icon.typeID, icon.nameID, index+1)
		}
	}
	group := resources[imageCount]
	if wantSize := 6 + 14*imageCount; group.typeID != resourceTypeGroupIcon || len(group.data) != wantSize {
		t.Errorf("group is type %d with %d bytes, want RT_GROUP_ICON with %d", group.typeID, len(group.data), wantSize)
	}
}
