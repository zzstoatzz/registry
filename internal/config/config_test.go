package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToWellKnownConfig(t *testing.T) {
	config := &Config{
		VerificationInitialBackoffMs: 500,
		VerificationMaxBackoffMs:     10000,
		VerificationMaxRetries:       3,
		VerificationRequestTimeoutMs: 15000,
	}

	wellKnownConfig := config.ToWellKnownConfig()

	require.NotNil(t, wellKnownConfig)
	assert.Equal(t, 500*time.Millisecond, wellKnownConfig.InitialBackoff)
	assert.Equal(t, 10*time.Second, wellKnownConfig.MaxBackoff)
	assert.Equal(t, 3, wellKnownConfig.MaxRetries)
	assert.Equal(t, 15*time.Second, wellKnownConfig.RequestTimeout)
}

func TestConfig_ToWellKnownConfig_WithDefaults(t *testing.T) {
	config := &Config{
		// Using default values from struct tags
		VerificationInitialBackoffMs: 1000,
		VerificationMaxBackoffMs:     30000,
		VerificationMaxRetries:       5,
		VerificationRequestTimeoutMs: 30000,
	}

	wellKnownConfig := config.ToWellKnownConfig()

	require.NotNil(t, wellKnownConfig)
	assert.Equal(t, 1*time.Second, wellKnownConfig.InitialBackoff)
	assert.Equal(t, 30*time.Second, wellKnownConfig.MaxBackoff)
	assert.Equal(t, 5, wellKnownConfig.MaxRetries)
	assert.Equal(t, 30*time.Second, wellKnownConfig.RequestTimeout)
}
