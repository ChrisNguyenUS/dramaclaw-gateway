package lumixai

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const maxImageReferences = 5

type imageReference struct {
	Image string `json:"image"`
}

// imageGenerationRequest is the LumixAI POST /v1/images/generations JSON body.
type imageGenerationRequest struct {
	Model       string           `json:"model"`
	Prompt      string           `json:"prompt"`
	AspectRatio string           `json:"aspectRatio,omitempty"`
	Resolution  string           `json:"resolution,omitempty"`
	References  []imageReference `json:"references,omitempty"`
}

type imageGenerationResponse struct {
	Data []struct {
		DownloadURL string `json:"download_url"`
	} `json:"data"`
}

func buildImageRequest(request dto.ImageRequest, fetch func(string) (string, error)) (*imageGenerationRequest, error) {
	out := &imageGenerationRequest{Model: request.Model, Prompt: request.Prompt}
	if ratio, _ := request.Metadata["ratio"].(string); ratio != "" && !strings.EqualFold(ratio, "auto") {
		out.AspectRatio = ratio
	}
	if resolution, _ := request.Metadata["resolution"].(string); resolution != "" {
		out.Resolution = strings.ToLower(resolution) // LumixAI image tiers are 1k / 2k / 4k
	}
	var refs []string
	for _, raw := range [][]byte{request.Image, request.Images} {
		values, err := imageValues(raw)
		if err != nil {
			return nil, err
		}
		refs = append(refs, values...)
	}
	if len(refs) > maxImageReferences {
		return nil, fmt.Errorf("unsupported_media_combination: at most %d reference images", maxImageReferences)
	}
	for _, ref := range refs {
		dataURI, err := fetch(ref)
		if err != nil {
			return nil, fmt.Errorf("fetch reference image: %w", err)
		}
		out.References = append(out.References, imageReference{Image: dataURI})
	}
	return out, nil
}

// imageValues accepts a JSON string or JSON string array (the DC-Media `image` field shapes).
func imageValues(raw []byte) ([]string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	if strings.HasPrefix(text, "[") {
		var values []string
		if err := common.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("invalid image array: %w", err)
		}
		return values, nil
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("invalid image value: %w", err)
	}
	return []string{value}, nil
}

// downloadImages fetches every key-protected result file and returns base64 payloads.
func downloadImages(baseURL, key string, payload []byte, client *http.Client) ([]string, error) {
	var resp imageGenerationResponse
	if err := common.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("decode lumixai image response: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("empty_result: LumixAI returned no image")
	}
	images := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/"+strings.TrimLeft(item.DownloadURL, "/"), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-API-Key", key)
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("download image: HTTP %d", res.StatusCode)
		}
		images = append(images, base64.StdEncoding.EncodeToString(body))
	}
	return images, nil
}
