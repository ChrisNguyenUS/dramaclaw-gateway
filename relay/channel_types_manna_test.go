package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Manna: the LumixAI channel must be offered with both image (sync) and video (task) capability.
func TestRegisteredChannelTypesIncludesLumixAI(t *testing.T) {
	var found *ChannelTypeMetadata
	for _, item := range GetRegisteredChannelTypes() {
		if item.Type == constant.ChannelTypeLumixAI {
			item := item
			found = &item
		}
	}
	require.NotNil(t, found, "LumixAI channel type 1001 is not registered")
	assert.Equal(t, "lumixai", found.Provider)
	assert.Equal(t, "https://api.lumixai.live", found.DefaultBaseURL)
	assert.Contains(t, found.Capabilities, "image")
	assert.Contains(t, found.Capabilities, "video")
}
