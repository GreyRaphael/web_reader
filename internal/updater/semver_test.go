package updater

import "testing"

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.7.1", "v1.7.1", 0},
		{"1.7.1", "v1.7.1", 0},
		{"v1.7.1", "v1.7.2", -1},
		{"v1.7.2", "v1.7.1", 1},
		{"v1.7.1", "v1.8.0", -1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.7", "v1.7.0", 0},
		{"v1.7", "v1.7.1", -1},
		{"v1.7.1", "v1.7.1-rc1", 1},
		{"v1.7.1-beta", "v1.7.1", -1},
		{"v1.7.1-alpha", "v1.7.1-beta", -1},
		{"dev", "v1.7.1", -1},
		{"v1.7.1", "dev", 1},
		{"dev", "dev", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			got := CompareSemver(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("CompareSemver(%q, %q) = %d, expected %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}
