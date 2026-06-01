package cache

import (
	"github.com/spf13/viper"
)

// HotKeyConfig holds the sliding-window hotkey detector parameters.
type HotKeyConfig struct {
	WindowSeconds       int `mapstructure:"window_seconds"`        // sliding window total duration (default 60s)
	SegmentSeconds      int `mapstructure:"segment_seconds"`       // time slice granularity (default 10s)
	LevelLow            int `mapstructure:"level_low"`             // low heat threshold
	LevelMedium         int `mapstructure:"level_medium"`          // medium heat threshold
	LevelHigh           int `mapstructure:"level_high"`            // high heat threshold
	ExtendLowSeconds    int `mapstructure:"extend_low_seconds"`    // TTL extension for low heat
	ExtendMediumSeconds int `mapstructure:"extend_medium_seconds"` // TTL extension for medium heat
	ExtendHighSeconds   int `mapstructure:"extend_high_seconds"`   // TTL extension for high heat
}

// FeedCacheConfig holds TTL and sizing parameters for the three cache tiers.
type FeedCacheConfig struct {
	L0TTLSeconds  int `mapstructure:"l0_ttl_seconds"`  // fragment cache base TTL
	L0JitterSec   int `mapstructure:"l0_jitter_seconds"` // random jitter range
	L1TTLSeconds  int `mapstructure:"l1_ttl_seconds"`  // page skeleton base TTL
	L1JitterSec   int `mapstructure:"l1_jitter_seconds"` // random jitter range
	L2TTLSeconds  int `mapstructure:"l2_ttl_seconds"`  // local cache base TTL
	L2JitterSec   int `mapstructure:"l2_jitter_seconds"` // random jitter range
	L2MaxEntries  int `mapstructure:"l2_max_entries"`  // max entries in local cache
}

// CacheConfig groups all cache-related configuration.
type CacheConfig struct {
	HotKey HotKeyConfig    `mapstructure:"hotkey"`
	Feed   FeedCacheConfig `mapstructure:"feed"`
}

// LoadConfig reads the "cache" subtree from viper into a CacheConfig with
// sensible defaults.
func LoadConfig() CacheConfig {
	cfg := CacheConfig{
		HotKey: HotKeyConfig{
			WindowSeconds:       60,
			SegmentSeconds:      10,
			LevelLow:            50,
			LevelMedium:         200,
			LevelHigh:           500,
			ExtendLowSeconds:    20,
			ExtendMediumSeconds: 60,
			ExtendHighSeconds:   120,
		},
		Feed: FeedCacheConfig{
			L0TTLSeconds: 60,
			L0JitterSec:  30,
			L1TTLSeconds: 10,
			L1JitterSec:  10,
			L2TTLSeconds: 15,
			L2JitterSec:  10,
			L2MaxEntries: 1000,
		},
	}

	if err := viper.UnmarshalKey("cache", &cfg); err != nil {
		// Use defaults if the section is missing.
	}

	return cfg
}
