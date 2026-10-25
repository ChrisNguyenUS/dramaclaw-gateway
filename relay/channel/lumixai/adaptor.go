package lumixai

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const ChannelName = "lumixai"

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func lumixBaseURL(info *relaycommon.RelayInfo) string {
	if base := strings.TrimRight(info.ChannelBaseUrl, "/"); base != "" {
		return base
	}
	return constant.ChannelBaseURLs[constant.ChannelTypeLumixAI]
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations && info.RelayMode != relayconstant.RelayModeImagesEdits {
		return "", fmt.Errorf("lumixai: unsupported relay mode %d", info.RelayMode)
	}
	return lumixBaseURL(info) + "/v1/images/generations", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-API-Key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.IsModelMapped {
		request.Model = info.UpstreamModelName
	}
	return buildImageRequest(request, func(raw string) (string, error) {
		if strings.HasPrefix(raw, "data:") {
			return raw, nil
		}
		mimeType, data, err := service.GetImageFromUrl(raw)
		if err != nil {
			return "", err
		}
		return "data:" + mimeType + ";base64," + data, nil
	})
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse)
	}
	images, err := downloadImages(lumixBaseURL(info), info.ApiKey, body, client)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	out := dto.ImageResponse{Created: time.Now().Unix()}
	for _, b64 := range images {
		out.Data = append(out.Data, dto.ImageData{B64Json: b64})
	}
	payload, err := common.Marshal(out)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(payload)
	return &dto.Usage{}, nil
}

func (a *Adaptor) GetModelList() []string    { return []string{} }
func (a *Adaptor) GetChannelName() string    { return ChannelName }
func (a *Adaptor) GetCapabilities() []string { return []string{"image"} }

var errNotImplemented = errors.New("lumixai adaptor: not implemented")

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errNotImplemented
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errNotImplemented
}
