package plugin

import (
	"testing"
)

func TestValidateCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		rigVersion  string
		requirement string
		wantStatus  Status
	}{
		{
			name:        "compatible version",
			rigVersion:  "1.0.0",
			requirement: ">= 1.0.0",
			wantStatus:  StatusCompatible,
		},
		{
			name:        "incompatible version",
			rigVersion:  "0.9.0",
			requirement: ">= 1.0.0",
			wantStatus:  StatusIncompatible,
		},
		{
			name:        "dev version always compatible",
			rigVersion:  "dev",
			requirement: ">= 1.0.0",
			wantStatus:  StatusCompatible,
		},
		{
			name:        "empty requirement compatible",
			rigVersion:  "1.0.0",
			requirement: "",
			wantStatus:  StatusCompatible,
		},
		{
			name:        "invalid requirement constraint",
			rigVersion:  "1.0.0",
			requirement: "invalid",
			wantStatus:  StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				Status: StatusCompatible,
				Manifest: &Manifest{
					Requirements: struct {
						Rig string `yaml:"rig"`
					}{Rig: tt.requirement},
				},
			}
			err := ValidateCompatibility(p, tt.rigVersion)
			if p.Status != tt.wantStatus {
				t.Errorf("ValidateCompatibility() status = %v, want %v (err: %v)", p.Status, tt.wantStatus, err)
			}
		})
	}
}
