package screens

import "testing"

func TestPackageDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{name: "has description", description: "A great package", want: "A great package"},
		{
			name:        "empty falls back to placeholder",
			description: "",
			want:        "No description available.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packageDescription(tt.description); got != tt.want {
				t.Errorf("packageDescription(%q) = %q, want %q", tt.description, got, tt.want)
			}
		})
	}
}
