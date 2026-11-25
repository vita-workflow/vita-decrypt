//go:build linux || windows

package desktop

import (
	"reflect"
	"testing"
)

func TestWithAllFiles(t *testing.T) {
	ageFiles := Filter{Name: "age files", Patterns: []string{"*.age"}}
	tests := []struct {
		name        string
		filters     []Filter
		wantFilters []Filter
	}{
		{name: "no filters stay none", filters: nil, wantFilters: nil},
		{
			name:    "filters get All files last",
			filters: []Filter{ageFiles},
			wantFilters: []Filter{
				ageFiles,
				{Name: allFilesName, Patterns: []string{"*.*"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if filters := withAllFiles(test.filters, "*.*"); !reflect.DeepEqual(filters, test.wantFilters) {
				t.Fatalf("withAllFiles = %v, want %v", filters, test.wantFilters)
			}
		})
	}
}

func TestWithAllFiles_LeavesTheCallersSliceAlone(t *testing.T) {
	backing := make([]Filter, 2)
	backing[0] = Filter{Name: "age files", Patterns: []string{"*.age"}}
	withAllFiles(backing[:1], "*")
	if backing[1].Name != "" {
		t.Fatalf("withAllFiles wrote %v past the end of the caller's slice", backing[1])
	}
}
