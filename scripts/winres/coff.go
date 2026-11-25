package main

import (
	"cmp"
	"slices"
)

// Windows resource types.
const (
	resourceTypeIcon      = 3  // RT_ICON
	resourceTypeGroupIcon = 14 // RT_GROUP_ICON
	resourceTypeVersion   = 16 // RT_VERSION
)

// Sizes of the PE/COFF structures, in bytes.
const (
	fileHeaderSize        = 20 // IMAGE_FILE_HEADER
	sectionHeaderSize     = 40 // IMAGE_SECTION_HEADER
	directoryHeaderSize   = 16 // IMAGE_RESOURCE_DIRECTORY
	directoryEntrySize    = 8  // IMAGE_RESOURCE_DIRECTORY_ENTRY
	dataEntrySize         = 16 // IMAGE_RESOURCE_DATA_ENTRY
	languageDirectorySize = directoryHeaderSize + directoryEntrySize
)

// PE/COFF field values.
const (
	machineAMD64           = 0x8664
	fileLineNumsStripped   = 0x4        // IMAGE_FILE_LINE_NUMS_STRIPPED
	sectionCharacteristics = 0x40300040 // initialized data, 4-byte aligned, readable
	relocationADDR32NB     = 3          // IMAGE_REL_AMD64_ADDR32NB
	symbolClassStatic      = 3          // IMAGE_SYM_CLASS_STATIC
	subdirectoryFlag       = 0x80000000 // set on a directory entry that points at another directory
	sectionName            = ".rsrc\x00\x00\x00"
	resourceDataAlignment  = 8
)

// resource is one Windows resource, identified by its type and its numeric ID.
type resource struct {
	typeID uint16
	nameID uint16
	data   []byte
}

// resourceGroup is the resources of one type, sorted by ID.
type resourceGroup struct {
	typeID    uint16
	resources []resource
}

// encodeCOFF lays out resources the way windres does: one .rsrc section holding the resource directory (types, then
// each type's IDs, each followed by its language directories), the data entries, and the data itself, each aligned to
// 8 bytes; one ADDR32NB relocation per data entry against the section's symbol; and an empty string table.
func encodeCOFF(resources []resource) ([]byte, error) {
	sorted := slices.SortedStableFunc(slices.Values(resources), func(first, second resource) int {
		return cmp.Or(cmp.Compare(first.typeID, second.typeID), cmp.Compare(first.nameID, second.nameID))
	})
	var section littleEndianWriter
	entriesOffset := appendDirectories(&section, groupByType(sorted))
	relocations := appendDataEntries(&section, sorted, entriesOffset)
	for _, resource := range sorted {
		section.appendBytes(resource.data)
		section.pad(resourceDataAlignment)
	}
	if section.err != nil {
		return nil, section.err
	}
	if relocations.err != nil {
		return nil, relocations.err
	}
	return encodeObjectFile(section.data, relocations.data, len(sorted))
}

// groupByType splits sorted resources into one group per type.
func groupByType(sorted []resource) []resourceGroup {
	var groups []resourceGroup
	for _, resource := range sorted {
		if len(groups) == 0 || groups[len(groups)-1].typeID != resource.typeID {
			groups = append(groups, resourceGroup{typeID: resource.typeID})
		}
		last := &groups[len(groups)-1]
		last.resources = append(last.resources, resource)
	}
	return groups
}

// appendDirectories appends the resource directory tree and returns its size, which is where the data entries start.
// The root lists each type's ID directory; each ID directory is followed by its language directories, whose single
// entries point at the data entries in order.
func appendDirectories(section *littleEndianWriter, groups []resourceGroup) int {
	rootSize := directoryHeaderSize + directoryEntrySize*len(groups)
	appendDirectoryHeader(section, len(groups))
	offset := rootSize
	for _, group := range groups {
		appendDirectoryEntry(section, group.typeID, offset, true)
		offset += directoryHeaderSize + (directoryEntrySize+languageDirectorySize)*len(group.resources)
	}
	entriesOffset := offset

	offset = rootSize
	dataEntryIndex := 0
	for _, group := range groups {
		appendDirectoryHeader(section, len(group.resources))
		languagesOffset := offset + directoryHeaderSize + directoryEntrySize*len(group.resources)
		for index, resource := range group.resources {
			appendDirectoryEntry(section, resource.nameID, languagesOffset+languageDirectorySize*index, true)
		}
		for range group.resources {
			appendDirectoryHeader(section, 1)
			appendDirectoryEntry(section, languageEnglishUS, entriesOffset+dataEntrySize*dataEntryIndex, false)
			dataEntryIndex++
		}
		offset = languagesOffset + languageDirectorySize*len(group.resources)
	}
	return entriesOffset
}

// appendDirectoryHeader appends an IMAGE_RESOURCE_DIRECTORY with entryCount entries, all identified by ID.
func appendDirectoryHeader(section *littleEndianWriter, entryCount int) {
	section.appendBytes(make([]byte, 12)) // characteristics, time stamp, major and minor version
	section.appendUint16(0)               // entries identified by name
	section.appendIntAsUint16(entryCount, "resource directory entry count")
}

// appendDirectoryEntry appends an IMAGE_RESOURCE_DIRECTORY_ENTRY for entryID that points at offset, which is another
// directory when isSubdirectory is true and a data entry otherwise.
func appendDirectoryEntry(section *littleEndianWriter, entryID uint16, offset int, isSubdirectory bool) {
	section.appendUint32(uint32(entryID))
	offsetField, err := toUint32(offset, "resource directory offset")
	section.recordError(err)
	if isSubdirectory {
		offsetField |= subdirectoryFlag
	}
	section.appendUint32(offsetField)
}

// appendDataEntries appends an IMAGE_RESOURCE_DATA_ENTRY for each resource, whose data follows the entries in the same
// order, and returns the relocations that turn each entry's data offset into an RVA when the program is linked.
func appendDataEntries(section *littleEndianWriter, sorted []resource, entriesOffset int) *littleEndianWriter {
	var relocations littleEndianWriter
	dataOffset := entriesOffset + dataEntrySize*len(sorted)
	for index, resource := range sorted {
		relocations.appendIntAsUint32(entriesOffset+dataEntrySize*index, "relocation offset")
		relocations.appendUint32(0) // symbol 0, .rsrc
		relocations.appendUint16(relocationADDR32NB)
		section.appendIntAsUint32(dataOffset, "resource data offset")
		section.appendIntAsUint32(len(resource.data), "resource size")
		section.appendBytes(make([]byte, 8)) // code page, reserved
		dataOffset += alignUp(len(resource.data), resourceDataAlignment)
	}
	return &relocations
}

// encodeObjectFile returns the COFF object file holding the .rsrc section: the file header, the section header, the
// section, its relocations, the section's symbol, and an empty string table.
func encodeObjectFile(section, relocations []byte, relocationCount int) ([]byte, error) {
	const headersSize = fileHeaderSize + sectionHeaderSize
	relocationsOffset := headersSize + len(section)
	symbolTableOffset := relocationsOffset + len(relocations)
	var object littleEndianWriter
	object.appendUint16(machineAMD64)
	object.appendUint16(1) // sections
	object.appendUint32(0) // time stamp
	object.appendIntAsUint32(symbolTableOffset, "symbol table offset")
	object.appendUint32(1) // symbols
	object.appendUint16(0) // optional header size
	object.appendUint16(fileLineNumsStripped)

	object.appendBytes([]byte(sectionName))
	object.appendUint32(0) // virtual size
	object.appendUint32(0) // virtual address
	object.appendIntAsUint32(len(section), "section size")
	object.appendUint32(headersSize) // raw data offset
	object.appendIntAsUint32(relocationsOffset, "relocations offset")
	object.appendUint32(0) // line numbers offset
	object.appendIntAsUint16(relocationCount, "relocation count")
	object.appendUint16(0) // line number count
	object.appendUint32(sectionCharacteristics)

	object.appendBytes(section)
	object.appendBytes(relocations)

	object.appendBytes([]byte(sectionName))
	object.appendUint32(0)                           // value
	object.appendUint16(1)                           // section number
	object.appendUint16(0)                           // type
	object.appendBytes([]byte{symbolClassStatic, 0}) // storage class, auxiliary symbol count
	object.appendUint32(4)                           // string table: just its own size
	if object.err != nil {
		return nil, object.err
	}
	return object.data, nil
}
