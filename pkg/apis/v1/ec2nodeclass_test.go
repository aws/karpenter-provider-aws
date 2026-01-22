package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCalculateVolumeSize(t *testing.T) {
	tests := []struct {
		name        string
		volumeSize  string
		cpuCount    int
		expectedGiB int64
		expectError bool
	}{
		{
			name:        "Static volume size",
			volumeSize:  "20Gi",
			cpuCount:    4,
			expectedGiB: 20,
			expectError: false,
		},
		{
			name:        "Dynamic volume size with CPU multiplier",
			volumeSize:  "10Gi * CPU",
			cpuCount:    4,
			expectedGiB: 40,
			expectError: false,
		},
		{
			name:        "Invalid formula",
			volumeSize:  "invalid",
			cpuCount:    4,
			expectedGiB: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock EC2NodeClass with BlockDeviceMappings
			nodeClass := &EC2NodeClass{
				Spec: EC2NodeClassSpec{
					BlockDeviceMappings: []*BlockDeviceMapping{
						{
							EBS: &BlockDevice{
								VolumeSize: func() *resource.Quantity {
									q := resource.MustParse(tt.volumeSize)
									return &q
								}(),
							},
						},
					},
				},
			}

			// Call CalculateVolumeSize
			result := nodeClass.CalculateVolumeSize(tt.cpuCount)

			// Validate result
			if result != nil && result.Value() != tt.expectedGiB*1024*1024*1024 {
				t.Errorf("expected %dGi, got %dGi", tt.expectedGiB, result.Value()/(1024*1024*1024))
			}

			// Check for errors in invalid cases
			if tt.expectError && result != nil {
				t.Errorf("expected error, but got result: %dGi", result.Value()/(1024*1024*1024))
			}
		})
	}
}
