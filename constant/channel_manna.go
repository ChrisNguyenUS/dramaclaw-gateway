package constant

// Manna Studio provider ids. Reserved far above upstream's contiguous range so new upstream
// providers can never collide with stored channels.type values. See
// docs/adr/0002-reserved-channel-ids.md in the Manna Studio repo.
const (
	ChannelTypeLumixAI = 1001
	ChannelTypeKIE     = 1002 // reserved for P4

	APITypeLumixAI = 1001
)

// mannaChannelTypes lists the Manna channel types that are live in this build.
var mannaChannelTypes = []int{ChannelTypeLumixAI}

func init() {
	for len(ChannelBaseURLs) <= ChannelTypeKIE {
		ChannelBaseURLs = append(ChannelBaseURLs, "")
	}
	ChannelBaseURLs[ChannelTypeLumixAI] = "https://api.lumixai.live"
	ChannelTypeNames[ChannelTypeLumixAI] = "LumixAI"
}

// RegisteredChannelTypes returns upstream's contiguous channel types plus Manna's live ones.
// Use it instead of looping 1..ChannelTypeDummy.
func RegisteredChannelTypes() []int {
	types := make([]int, 0, ChannelTypeDummy+len(mannaChannelTypes))
	for channelType := 1; channelType <= ChannelTypeDummy; channelType++ {
		types = append(types, channelType)
	}
	return append(types, mannaChannelTypes...)
}
