package randomdata

import (
	"testing"
)

func Test_FullProfileGenerator(t *testing.T) {
	profile := GenerateProfile(1)

	if profile.Gender != "female" {
		t.Fatalf("Expected gender to be female but got %s", profile.Gender)
	}

	profile = GenerateProfile(0)

	if profile.Gender != "male" {
		t.Fatalf("Expected gender to be male but got %s", profile.Gender)
	}

	profile = GenerateProfile(2)

	if profile == nil {
		t.Fatal("Profile failed to generate")
	}

	if !CheckPhoneNumber(profile.Cell, t) {
		t.Fatalf("Expected Cell# to be a valid phone number: %v", profile.Cell)
	}

	if !CheckPhoneNumber(profile.Phone, t) {
		t.Fatalf("Expected Phone# to be a valid phone number: %v", profile.Phone)
	}

	if profile.Login.Username == "" {
		t.Fatal("Profile Username failed to generate")
	}

	if profile.Location.Street == "" {
		t.Fatal("Profile Street failed to generate")
	}

	if profile.ID.Name != "SSN" {
		t.Fatalf("Profile ID Name to be SSN, but got %s\n", profile.ID.Name)
	}

	if profile.Picture.Large == "" {
		t.Fatalf("Profile Picture Large to be set, but got %s\n", profile.Picture.Large)
	}
}
