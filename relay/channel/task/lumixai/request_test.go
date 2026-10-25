package lumixai

import (
	"errors"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeFetch(raw string) (string, error) { return "data:image/png;base64,FAKE(" + raw + ")", nil }

func TestBuildVideoJobMapsFirstAndLastFrame(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model: "google-veo3_1-lite", Prompt: "turn to camera", Image: "https://x/first.png", Duration: 6,
		Metadata: map[string]any{"last_frame_image": "https://x/last.png", "ratio": "auto", "resolution": "720p", "generate_audio": true},
	}
	job, err := buildVideoJob(req, fakeFetch)
	require.NoError(t, err)
	assert.Equal(t, "google-veo3_1-lite", job.Model)
	assert.Equal(t, "data:image/png;base64,FAKE(https://x/first.png)", job.Image)
	assert.Equal(t, "data:image/png;base64,FAKE(https://x/last.png)", job.EndImage)
	assert.Empty(t, job.Images)
	require.NotNil(t, job.Duration)
	assert.Equal(t, 6, *job.Duration)
	require.NotNil(t, job.WithSoundEffects)
	assert.True(t, *job.WithSoundEffects)
}

func TestBuildVideoJobKeepsSingleReferenceAsReference(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model: "kling-30", Prompt: "p",
		Metadata: map[string]any{"reference_images": []string{"https://x/ref.png"}, "ratio": "9:16"},
	}
	job, err := buildVideoJob(req, fakeFetch)
	require.NoError(t, err)
	assert.Empty(t, job.Image, "a reference image must never become the first frame")
	assert.Equal(t, []string{"data:image/png;base64,FAKE(https://x/ref.png)"}, job.Images)
	require.NotNil(t, job.MaxRefs)
	assert.Equal(t, 1, *job.MaxRefs)
	assert.Equal(t, "9:16", job.AspectRatio)
}

func TestBuildVideoJobOmitsAutoRatioAndUppercasesK(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "kling-30", Prompt: "p", Metadata: map[string]any{"ratio": "auto", "resolution": "4k"}}
	job, err := buildVideoJob(req, fakeFetch)
	require.NoError(t, err)
	assert.Empty(t, job.AspectRatio)
	assert.Equal(t, "4K", job.Resolution)
	assert.Nil(t, job.WithSoundEffects, "absent generate_audio must stay absent")
	assert.Nil(t, job.Duration)
}

func TestBuildVideoJobPreservesExplicitFalseAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Metadata: map[string]any{"generate_audio": false}}
	job, err := buildVideoJob(req, fakeFetch)
	require.NoError(t, err)
	require.NotNil(t, job.WithSoundEffects)
	assert.False(t, *job.WithSoundEffects)
}

func TestValidateVideoMediaShapeRejectsUnsupported(t *testing.T) {
	cases := map[string]relaycommon.TaskSubmitReq{
		"reference video": {Model: "m", Prompt: "p", Metadata: map[string]any{"reference_videos": []string{"https://x/a.mp4"}}},
		"reference audio": {Model: "m", Prompt: "p", Metadata: map[string]any{"reference_audios": []string{"https://x/a.mp3"}}},
		"reference file":  {Model: "m", Prompt: "p", Metadata: map[string]any{"reference_file": "https://x/a.pdf"}},
		"reference link":  {Model: "m", Prompt: "p", Metadata: map[string]any{"reference_link": "https://x"}},
		"auto duration":   {Model: "m", Prompt: "p", DurationAuto: true},
		"extend":          {Model: "m", Prompt: "p", Metadata: map[string]any{"omni_reference_task_type": "extend"}},
		"six references":  {Model: "m", Prompt: "p", Metadata: map[string]any{"reference_images": []string{"1", "2", "3", "4", "5", "6"}}},
		"first+refs":      {Model: "m", Prompt: "p", Image: "https://x/f.png", Metadata: map[string]any{"reference_images": []string{"https://x/r.png"}}},
		"last+refs":       {Model: "m", Prompt: "p", Metadata: map[string]any{"last_frame_image": "https://x/l.png", "reference_images": []string{"https://x/r.png"}}},
		"too long":        {Model: "m", Prompt: "p", Duration: relaycommon.MaxTaskDurationSeconds + 1},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateVideoMediaShape(req)
			var unsupported *unsupportedMediaError
			require.Error(t, err)
			assert.True(t, errors.As(err, &unsupported), "want unsupportedMediaError, got %T", err)
		})
	}
}

func TestValidateVideoMediaShapeAcceptsTextOnly(t *testing.T) {
	assert.NoError(t, validateVideoMediaShape(relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Duration: 5}))
}

func TestBuildVideoJobPropagatesFetchError(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Image: "https://x/f.png"}
	_, err := buildVideoJob(req, func(string) (string, error) { return "", errors.New("boom") })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "first frame")
}
