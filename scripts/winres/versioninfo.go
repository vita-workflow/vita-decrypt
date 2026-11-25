package main

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf16"
)

// versionDetails is the text of the version info.
type versionDetails struct {
	version     string
	description string
	programName string
	product     string
	company     string
	copyright   string
}

// fileVersion is a version as VS_FIXEDFILEINFO holds it: four numbers, and whether it's a pre-release.
type fileVersion struct {
	numbers      [4]uint16
	isPrerelease bool
}

var (
	validVersion    = regexp.MustCompile(`^[0-9A-Za-z.+-]+$`)
	semanticVersion = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-.+)?$`)
)

// VS_FIXEDFILEINFO values.
const (
	fixedFileInfoSignature = 0xfeef04bd
	fixedFileInfoVersion   = 0x10000
	fileFlagsMask          = 0x3f
	fileFlagPrerelease     = 0x2     // VS_FF_PRERELEASE
	fileOSWindowsNT        = 0x40004 // VOS_NT_WINDOWS32
	fileTypeApp            = 0x1     // VFT_APP
)

// The language windres gives every resource (US English) and the code page of the version info's strings (Unicode).
const (
	languageEnglishUS = 0x409
	codePageUnicode   = 1200
)

// versionBlockAlignment is the alignment of each VS_VERSIONINFO block's value and children.
const versionBlockAlignment = 4

// parseVersion returns the four numbers Windows wants for version: 1.2.3-rc.1 becomes 1.2.3.0, and anything that isn't
// a semantic version (such as dev) 0.0.0.0. Pre-releases and anything that isn't a semantic version are flagged as
// pre-releases.
func parseVersion(version string) (fileVersion, error) {
	if !validVersion.MatchString(version) {
		return fileVersion{}, fmt.Errorf("version %q has characters other than letters, digits, '.', '+', and '-'",
			version)
	}
	match := semanticVersion.FindStringSubmatch(version)
	if match == nil {
		return fileVersion{isPrerelease: true}, nil
	}
	parsed := fileVersion{isPrerelease: match[4] != ""}
	for index, digits := range match[1:4] {
		number, err := strconv.ParseUint(digits, 10, 16)
		if err != nil {
			return fileVersion{}, fmt.Errorf("version %q: each number must be at most 65535", version)
		}
		parsed.numbers[index] = uint16(number)
	}
	return parsed, nil
}

// encodeVersionInfo returns the VS_VERSIONINFO resource for details.
func encodeVersionInfo(details versionDetails) ([]byte, error) {
	version, err := parseVersion(details.version)
	if err != nil {
		return nil, err
	}
	stringTable := versionNode{key: "040904b0", isText: true}
	for _, field := range []struct{ name, value string }{
		{"CompanyName", details.company},
		{"FileDescription", details.description},
		{"FileVersion", details.version},
		{"InternalName", details.programName},
		{"LegalCopyright", details.copyright},
		{"OriginalFilename", details.programName + ".exe"},
		{"ProductName", details.product},
		{"ProductVersion", details.version},
	} {
		stringTable.children = append(stringTable.children,
			versionNode{key: field.name, isText: true, value: nullTerminatedUTF16(field.value)})
	}
	translation := binary.LittleEndian.AppendUint16(nil, languageEnglishUS)
	translation = binary.LittleEndian.AppendUint16(translation, codePageUnicode)
	root := versionNode{
		key: "VS_VERSION_INFO", value: encodeFixedFileInfo(version),
		children: []versionNode{
			{key: "StringFileInfo", isText: true, children: []versionNode{stringTable}},
			{key: "VarFileInfo", isText: true, children: []versionNode{
				{key: "Translation", value: translation},
			}},
		},
	}
	return root.encode()
}

// encodeFixedFileInfo returns the VS_FIXEDFILEINFO structure for version.
func encodeFixedFileInfo(version fileVersion) []byte {
	mostSignificant := uint32(version.numbers[0])<<16 | uint32(version.numbers[1])
	leastSignificant := uint32(version.numbers[2])<<16 | uint32(version.numbers[3])
	flags := uint32(0)
	if version.isPrerelease {
		flags = fileFlagPrerelease
	}
	var fixed []byte
	for _, field := range []uint32{
		fixedFileInfoSignature, fixedFileInfoVersion,
		mostSignificant, leastSignificant, // file version
		mostSignificant, leastSignificant, // product version
		fileFlagsMask, flags,
		fileOSWindowsNT, fileTypeApp, 0, // no subtype
		0, 0, // file date
	} {
		fixed = binary.LittleEndian.AppendUint32(fixed, field)
	}
	return fixed
}

// versionNode is one block of a VS_VERSIONINFO tree. A text block's value is a NUL-terminated UTF-16 string; any other
// block's value is binary.
type versionNode struct {
	key      string
	isText   bool
	value    []byte
	children []versionNode
}

// encode returns the block: its length, its value's length (in UTF-16 units for text and bytes otherwise), its type,
// its key, its value, and its children.
func (node versionNode) encode() ([]byte, error) {
	valueLength := len(node.value)
	if node.isText {
		valueLength /= 2
	}
	valueLengthField, err := toUint16(valueLength, fmt.Sprintf("length of the version info value %q", node.key))
	if err != nil {
		return nil, err
	}
	block := make([]byte, 6, 64) // length, value length, and type, filled in below
	binary.LittleEndian.PutUint16(block[2:], valueLengthField)
	if node.isText {
		binary.LittleEndian.PutUint16(block[4:], 1)
	}
	block = append(block, nullTerminatedUTF16(node.key)...)
	block = pad(block, versionBlockAlignment)
	block = append(block, node.value...)
	for _, child := range node.children {
		encodedChild, err := child.encode()
		if err != nil {
			return nil, err
		}
		block = pad(block, versionBlockAlignment)
		block = append(block, encodedChild...)
	}
	lengthField, err := toUint16(len(block), fmt.Sprintf("length of the version info block %q", node.key))
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint16(block[0:], lengthField)
	return block, nil
}

// nullTerminatedUTF16 returns text as NUL-terminated little-endian UTF-16.
func nullTerminatedUTF16(text string) []byte {
	var encoded []byte
	for _, unit := range utf16.Encode([]rune(text + "\x00")) {
		encoded = binary.LittleEndian.AppendUint16(encoded, unit)
	}
	return encoded
}
