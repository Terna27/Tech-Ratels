package geodata_test

import (
	"strings"
	"testing"

	"ratels-api/internal/geodata"
)

func TestGeodata_CompletenessAndInvariants(t *testing.T) {
	zones := geodata.Zones()
	if len(zones) != geodata.ExpectedZonesCount {
		t.Fatalf("expected %d zones, got %d", geodata.ExpectedZonesCount, len(zones))
	}

	zoneCodes := make(map[string]bool)
	for _, z := range zones {
		if z.Code == "" || z.Name == "" {
			t.Errorf("empty zone code or name: %+v", z)
		}
		if zoneCodes[z.Code] {
			t.Errorf("duplicate zone code: %s", z.Code)
		}
		zoneCodes[z.Code] = true
	}

	states := geodata.States()
	if len(states) != geodata.ExpectedStatesCount {
		t.Fatalf("expected %d states (36 states + FCT), got %d", geodata.ExpectedStatesCount, len(states))
	}

	stateCodes := make(map[string]bool)
	stateNames := make(map[string]bool)
	fctCount := 0
	totalLGAs := 0
	stateLGAs := 0

	for _, s := range states {
		if s.Code == "" || s.Name == "" || s.CapitalCity == "" {
			t.Errorf("incomplete state record: %+v", s)
		}
		if stateCodes[s.Code] {
			t.Errorf("duplicate state code: %s", s.Code)
		}
		stateCodes[s.Code] = true

		if stateNames[s.Name] {
			t.Errorf("duplicate state name: %s", s.Name)
		}
		stateNames[s.Name] = true

		if !zoneCodes[s.ZoneCode] {
			t.Errorf("state %s (%s) assigned to non-existent zone %s", s.Name, s.Code, s.ZoneCode)
		}

		lgaNames := make(map[string]bool)
		for _, l := range s.LGAs {
			trimmed := strings.TrimSpace(l)
			if trimmed == "" {
				t.Errorf("empty LGA name in state %s", s.Name)
			}
			lower := strings.ToLower(trimmed)
			if lgaNames[lower] {
				t.Errorf("duplicate LGA %s in state %s", trimmed, s.Name)
			}
			lgaNames[lower] = true
		}

		totalLGAs += len(s.LGAs)
		if s.IsFCT {
			fctCount++
			if s.Code != "FC" {
				t.Errorf("expected FCT code to be FC, got %s", s.Code)
			}
			if len(s.LGAs) != geodata.ExpectedAreaCouncils {
				t.Errorf("expected FCT to have %d area councils, got %d", geodata.ExpectedAreaCouncils, len(s.LGAs))
			}
		} else {
			stateLGAs += len(s.LGAs)
		}
	}

	if fctCount != 1 {
		t.Errorf("expected exactly 1 FCT record, got %d", fctCount)
	}

	if totalLGAs != geodata.ExpectedTotalLGAs {
		t.Errorf("expected %d total administrative units, got %d", geodata.ExpectedTotalLGAs, totalLGAs)
	}

	if stateLGAs != geodata.ExpectedStateLGAs {
		t.Errorf("expected %d state LGAs, got %d", geodata.ExpectedStateLGAs, stateLGAs)
	}

	if geodata.TotalLGACount() != geodata.ExpectedTotalLGAs {
		t.Errorf("TotalLGACount() returned %d, expected %d", geodata.TotalLGACount(), geodata.ExpectedTotalLGAs)
	}
}
