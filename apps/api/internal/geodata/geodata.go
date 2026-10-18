package geodata

const (
	AuthoritativeSource  = "Constitution of the Federal Republic of Nigeria 1999 (First Schedule, Part I) / National Bureau of Statistics (NBS) 2018"
	VerificationDate     = "2026-10-18"
	ExpectedZonesCount   = 6
	ExpectedStatesCount  = 37 // 36 States + Federal Capital Territory
	ExpectedTotalLGAs    = 774
	ExpectedStateLGAs    = 768
	ExpectedAreaCouncils = 6
)

// ZoneInfo defines a Nigerian geopolitical zone.
type ZoneInfo struct {
	Code string
	Name string
}

// StateInfo defines a Nigerian state or territory along with its capital city and official LGAs / area councils.
type StateInfo struct {
	ZoneCode    string
	Code        string
	Name        string
	CapitalCity string
	IsFCT       bool
	LGAs        []string
}

// Zones returns the authoritative list of all six Nigerian geopolitical zones.
func Zones() []ZoneInfo {
	return []ZoneInfo{
		{Code: "NC", Name: "North Central"},
		{Code: "NE", Name: "North East"},
		{Code: "NW", Name: "North West"},
		{Code: "SE", Name: "South East"},
		{Code: "SS", Name: "South South"},
		{Code: "SW", Name: "South West"},
	}
}

// TotalLGACount calculates the total number of administrative units across all states and the FCT.
func TotalLGACount() int {
	total := 0
	for _, s := range States() {
		total += len(s.LGAs)
	}
	return total
}
