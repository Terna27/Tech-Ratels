package service_test

import (
	"context"
	"testing"
)

func TestGeographyHierarchy(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. List Zones
	zones, err := env.GeoService.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones returned error: %v", err)
	}

	if len(zones) < 6 {
		t.Errorf("Expected at least 6 geopolitical zones, got %d", len(zones))
	}

	expectedZoneCodes := map[string]bool{
		"NC": false, "NE": false, "NW": false,
		"SE": false, "SS": false, "SW": false,
	}
	for _, z := range zones {
		if _, ok := expectedZoneCodes[z.Code]; ok {
			expectedZoneCodes[z.Code] = true
		}
	}
	for code, found := range expectedZoneCodes {
		if !found {
			t.Errorf("Zone code %s was not found in database", code)
		}
	}

	// 2. List States
	states, err := env.GeoService.ListStates(ctx)
	if err != nil {
		t.Fatalf("ListStates returned error: %v", err)
	}

	if len(states) < 37 {
		t.Errorf("Expected at least 37 state/territory records (36 states + 1 FCT), got %d", len(states))
	}

	// 3. Verify FCT is explicitly modeled as a territory, not a 37th state
	var fctFound bool
	for _, s := range states {
		if s.Code == "FC" {
			fctFound = true
			if !s.IsFCT {
				t.Errorf("Expected FCT record to have IsFCT=true, got false")
			}
			if s.Name != "Federal Capital Territory" {
				t.Errorf("Expected FCT name 'Federal Capital Territory', got %s", s.Name)
			}
			break
		}
	}
	if !fctFound {
		t.Errorf("FCT record with code 'FC' was not found")
	}

	// 4. Verify LGAs beneath State
	var lagosID string
	for _, s := range states {
		if s.Code == "LA" {
			lagosID = s.ID
			break
		}
	}
	if lagosID == "" {
		t.Fatalf("Lagos state ID not found")
	}

	lagosLGAs, err := env.GeoService.ListLGAsByState(ctx, lagosID)
	if err != nil {
		t.Fatalf("ListLGAsByState returned error: %v", err)
	}
	if len(lagosLGAs) == 0 {
		t.Errorf("Expected LGAs for Lagos state, got 0")
	}
}
