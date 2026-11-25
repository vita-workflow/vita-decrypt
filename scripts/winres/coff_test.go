package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestEncodeCOFF_DoesNotChangeItsInput(t *testing.T) {
	resources := []resource{
		{typeID: resourceTypeVersion, nameID: 1, data: []byte{1}},
		{typeID: resourceTypeIcon, nameID: 2, data: []byte{2}},
		{typeID: resourceTypeIcon, nameID: 1, data: []byte{3}},
	}
	original := []resource{resources[0], resources[1], resources[2]}
	if _, err := encodeCOFF(resources); err != nil {
		t.Fatal(err)
	}
	for index, got := range resources {
		want := original[index]
		if got.typeID != want.typeID || got.nameID != want.nameID || !bytes.Equal(got.data, want.data) {
			t.Errorf("resource %d changed from %+v to %+v", index, want, got)
		}
	}
}

func TestEncodeCOFF_RejectsTooManyResourcesOfOneType(t *testing.T) {
	resources := make([]resource, 0, math.MaxUint16+1)
	for nameID := range uint16(math.MaxUint16) {
		resources = append(resources, resource{typeID: resourceTypeIcon, nameID: nameID})
	}
	resources = append(resources, resource{typeID: resourceTypeIcon, nameID: math.MaxUint16})
	_, err := encodeCOFF(resources)
	if err == nil || !strings.Contains(err.Error(), "resource directory entry count is 65536") {
		t.Errorf("error = %v, want one about the directory entry count", err)
	}
}
