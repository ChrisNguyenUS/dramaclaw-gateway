package lumixai

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildImageRequestLowercasesResolution(t *testing.T) {
	req := dto.ImageRequest{Model: "imagen-nano-banana-2-flash", Prompt: "p", Metadata: map[string]any{"ratio": "16:9", "resolution": "2K"}}
	out, err := buildImageRequest(req, func(s string) (string, error) { return s, nil })
	require.NoError(t, err)
	assert.Equal(t, "2k", out.Resolution)
	assert.Equal(t, "16:9", out.AspectRatio)
	assert.Empty(t, out.References)
}

func TestBuildImageRequestMapsEditImagesToReferences(t *testing.T) {
	req := dto.ImageRequest{Model: "flux-kontext", Prompt: "keep identity", Image: []byte(`["https://x/a.png","https://x/b.png"]`), Metadata: map[string]any{"ratio": "auto"}}
	out, err := buildImageRequest(req, func(s string) (string, error) { return "data:image/png;base64,F(" + s + ")", nil })
	require.NoError(t, err)
	assert.Empty(t, out.AspectRatio, "auto ratio is omitted")
	require.Len(t, out.References, 2)
	assert.Equal(t, "data:image/png;base64,F(https://x/a.png)", out.References[0].Image)
}

func TestBuildImageRequestRejectsSixReferences(t *testing.T) {
	req := dto.ImageRequest{Model: "m", Prompt: "p", Image: []byte(`["1","2","3","4","5","6"]`)}
	_, err := buildImageRequest(req, func(s string) (string, error) { return s, nil })
	require.Error(t, err)
}

func TestDownloadImagesSendsKeyAndReturnsBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "k1", r.Header.Get("X-API-Key"))
		assert.Equal(t, "/v1/images/results/abc/download", r.URL.Path)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer server.Close()
	payload := []byte(`{"created":1,"data":[{"download_url":"/v1/images/results/abc/download"}]}`)
	images, err := downloadImages(server.URL, "k1", payload, server.Client())
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("PNGDATA")), images[0])
}

func TestDownloadImagesFailsOnEmptyData(t *testing.T) {
	_, err := downloadImages("https://unused", "k", []byte(`{"created":1,"data":[]}`), http.DefaultClient)
	require.Error(t, err)
}
