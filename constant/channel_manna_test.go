package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLumixAIChannelRegistration(t *testing.T) {
	require.Greater(t, len(ChannelBaseURLs), ChannelTypeKIE)
	assert.Equal(t, "https://api.lumixai.live", ChannelBaseURLs[ChannelTypeLumixAI])
	assert.Equal(t, "LumixAI", GetChannelTypeName(ChannelTypeLumixAI))
}

func TestRegisteredChannelTypesKeepsUpstreamRangeAndAddsManna(t *testing.T) {
	types := RegisteredChannelTypes()
	assert.Contains(t, types, ChannelTypeOpenAI)
	assert.Contains(t, types, ChannelTypeComfyUI)
	assert.Contains(t, types, ChannelTypeLumixAI)
	assert.NotContains(t, types, ChannelTypeKIE, "KIE is reserved but not shipped until P4")
}
