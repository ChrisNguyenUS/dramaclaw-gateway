package lumixai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initAdaptor() *TaskAdaptor {
	a := &TaskAdaptor{}
	a.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.lumixai.live/"}})
	return a
}

func TestParseTaskResultMapsStatuses(t *testing.T) {
	a := initAdaptor()
	cases := []struct {
		body       string
		wantStatus model.TaskStatus
		wantRemote string
		wantReason string
	}{
		{`{"id":"j1","status":"pending"}`, model.TaskStatusQueued, "", ""},
		{`{"id":"j1","status":"running"}`, model.TaskStatusInProgress, "", ""},
		{`{"id":"j1","status":"done","data":[{"download_url":"/v1/videos/results/j1/download"}]}`,
			model.TaskStatusSuccess, "https://api.lumixai.live/v1/videos/results/j1/download", ""},
		{`{"id":"j1","status":"error","error":"did not pass safety checks"}`, model.TaskStatusFailure, "", "did not pass safety checks"},
		{`{"id":"j1","status":"done","data":[]}`, model.TaskStatusFailure, "", "empty_result"},
	}
	for _, tc := range cases {
		info, err := a.ParseTaskResult([]byte(tc.body))
		require.NoError(t, err)
		assert.Equal(t, string(tc.wantStatus), info.Status, tc.body)
		assert.Equal(t, tc.wantRemote, info.RemoteUrl, tc.body)
		assert.Contains(t, info.Reason, tc.wantReason, tc.body)
	}
}

func TestDoRequestNormalizesAccepted(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader(`{"id":"j1","status":"pending"}`))}
	normalizeAccepted(resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestConvertToOpenAIVideoUsesPublicProxy(t *testing.T) {
	a := initAdaptor()
	task := &model.Task{TaskID: "task_pub", Status: model.TaskStatusSuccess, Progress: "100%"}
	task.PrivateData.UpstreamResultURL = "https://api.lumixai.live/v1/videos/results/j1/download"
	raw, err := a.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, common.Unmarshal(raw, &out))
	metadata, _ := out["metadata"].(map[string]any)
	url, _ := metadata["url"].(string)
	assert.Contains(t, url, "/v1/public/videos/task_pub/content")
	assert.NotContains(t, url, "lumixai.live", "DramaClaw must never receive the key-protected LumixAI URL")
}
