package delivery

import (
	"testing"
)

func TestNormalizeNigerianPhone(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantE164     string
		wantProvider string
		wantErr      bool
	}{
		{
			name:         "Standard local 11-digit 080",
			input:        "08031234567",
			wantE164:     "+2348031234567",
			wantProvider: "2348031234567",
			wantErr:      false,
		},
		{
			name:         "Local with spaces and hyphens",
			input:        " 0803 - 123 - 4567 ",
			wantE164:     "+2348031234567",
			wantProvider: "2348031234567",
			wantErr:      false,
		},
		{
			name:         "International format with +234",
			input:        "+2348031234567",
			wantE164:     "+2348031234567",
			wantProvider: "2348031234567",
			wantErr:      false,
		},
		{
			name:         "International format without plus 234",
			input:        "2348031234567",
			wantE164:     "+2348031234567",
			wantProvider: "2348031234567",
			wantErr:      false,
		},
		{
			name:         "070 prefix",
			input:        "07011223344",
			wantE164:     "+2347011223344",
			wantProvider: "2347011223344",
			wantErr:      false,
		},
		{
			name:         "090 prefix",
			input:        "09099887766",
			wantE164:     "+2349099887766",
			wantProvider: "2349099887766",
			wantErr:      false,
		},
		{
			name:         "091 prefix",
			input:        "09122334455",
			wantE164:     "+2349122334455",
			wantProvider: "2349122334455",
			wantErr:      false,
		},
		{
			name:         "081 prefix",
			input:        "08188990011",
			wantE164:     "+2348188990011",
			wantProvider: "2348188990011",
			wantErr:      false,
		},
		{
			name:    "Too short",
			input:   "0803123",
			wantErr: true,
		},
		{
			name:    "Invalid prefix 020",
			input:   "02031234567",
			wantErr: true,
		},
		{
			name:    "Foreign number US",
			input:   "+15551234567",
			wantErr: true,
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Letters in phone",
			input:   "0803123ABCD",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotE164, gotProvider, err := NormalizeNigerianPhone(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeNigerianPhone(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if gotE164 != tt.wantE164 {
					t.Errorf("NormalizeNigerianPhone(%q) gotE164 = %v, want %v", tt.input, gotE164, tt.wantE164)
				}
				if gotProvider != tt.wantProvider {
					t.Errorf("NormalizeNigerianPhone(%q) gotProvider = %v, want %v", tt.input, gotProvider, tt.wantProvider)
				}
			}
		})
	}
}
