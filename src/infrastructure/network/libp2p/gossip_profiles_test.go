package libp2p

import (
	"testing"
)

func TestGetProfileForPeerCount(t *testing.T) {
	tests := []struct {
		count    int
		expected GossipProfile
	}{
		{1, ProfileSmall},
		{5, ProfileSmall},
		{10, ProfileSmall},
		{11, ProfileMedium},
		{25, ProfileMedium},
		{50, ProfileMedium},
		{51, ProfileLarge},
		{100, ProfileLarge},
		{200, ProfileLarge},
		{201, ProfileXLarge},
		{500, ProfileXLarge},
		{1000, ProfileXLarge},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := GetProfileForPeerCount(tt.count)
			if result != tt.expected {
				t.Errorf("GetProfileForPeerCount(%d) = %s, want %s", tt.count, result, tt.expected)
			}
		})
	}
}

func TestProfileParams(t *testing.T) {
	profiles := []GossipProfile{ProfileSmall, ProfileMedium, ProfileLarge, ProfileXLarge}

	for _, profile := range profiles {
		t.Run(string(profile), func(t *testing.T) {
			params, ok := ProfileParams[profile]
			if !ok {
				t.Fatalf("Profile %s not found in ProfileParams", profile)
			}

			// Validate constraints
			if params.D <= 0 {
				t.Errorf("D should be positive, got %d", params.D)
			}
			if params.Dlo >= params.D {
				t.Errorf("Dlo (%d) should be less than D (%d)", params.Dlo, params.D)
			}
			if params.Dhi <= params.D {
				t.Errorf("Dhi (%d) should be greater than D (%d)", params.Dhi, params.D)
			}
			if params.HeartbeatInterval <= 0 {
				t.Errorf("HeartbeatInterval should be positive")
			}
			if params.HistoryGossip <= 0 {
				t.Errorf("HistoryGossip should be positive")
			}
			if params.HistoryLength <= 0 {
				t.Errorf("HistoryLength should be positive")
			}
		})
	}
}

func TestGossipSubConfig_WithProfile(t *testing.T) {
	config := DefaultGossipSubConfig()

	// Change to large profile
	config = config.WithProfile(ProfileLarge)

	if config.Profile != ProfileLarge {
		t.Errorf("Profile should be ProfileLarge, got %s", config.Profile)
	}
	if config.Params.D != ProfileParams[ProfileLarge].D {
		t.Errorf("Params.D should match ProfileLarge params")
	}
}

func TestGossipSubConfig_ToOptions(t *testing.T) {
	config := DefaultGossipSubConfig()
	options := config.ToOptions()

	if len(options) == 0 {
		t.Error("Expected at least one option")
	}
}

func TestDefaultGossipParams(t *testing.T) {
	params := DefaultGossipParams()

	// Should match medium profile
	expected := ProfileParams[ProfileMedium]
	if params.D != expected.D {
		t.Errorf("Default D should be %d, got %d", expected.D, params.D)
	}
}
