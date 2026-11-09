package model

import "testing"

func TestValidateDeviceProfile(t *testing.T) {
	for _, p := range [][2]string{{"", ""}, {"legacy", "json"}, {"simple", "cbor"}, {"simple", ""}, {"", "json"}} {
		if err := ValidateDeviceProfile(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range [][2]string{{"dual", "json"}, {"simple", "xml"}, {"MQTT", "cbor"}} {
		if err := ValidateDeviceProfile(p[0], p[1]); err == nil {
			t.Fatalf("accepted invalid profile %v", p)
		}
	}
}
