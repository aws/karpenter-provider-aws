package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// InstanceType represents the structure of an instance type in the JSON file
type InstanceType struct {
	Name      string     `json:"name"`
	Offerings []Offering `json:"offerings"`
}

// Offering represents the pricing and availability information for an instance type
type Offering struct {
	Price        float64       `json:"Price"`
	Available    bool          `json:"Available"`
	Requirements []Requirement `json:"Requirements"`
}

// Requirement represents a requirement for an offering
type Requirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// LoadInstanceTypesFromFile loads instance types from a JSON file and returns a map of instance name to cost
func LoadInstanceTypesFromFile(filePath string) (map[string]float64, error) {
	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading instance types file: %v", err)
	}

	// Parse the JSON data
	var instanceTypes []InstanceType
	if err := json.Unmarshal(data, &instanceTypes); err != nil {
		return nil, fmt.Errorf("error parsing instance types file: %v", err)
	}

	// Create a map of instance name to cost
	// For each instance type, we'll use the on-demand price from the first available offering
	costMap := make(map[string]float64)
	for _, instanceType := range instanceTypes {
		// Find the first on-demand offering
		for _, offering := range instanceType.Offerings {
			// Check if this is an on-demand offering
			isOnDemand := false
			for _, req := range offering.Requirements {
				if req.Key == "karpenter.sh/capacity-type" {
					for _, val := range req.Values {
						if val == "on-demand" {
							isOnDemand = true
							break
						}
					}
				}
			}

			if isOnDemand && offering.Available {
				costMap[instanceType.Name] = offering.Price
				break
			}
		}
	}

	return costMap, nil
}
