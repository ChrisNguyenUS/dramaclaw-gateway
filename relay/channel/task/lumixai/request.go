package lumixai

import (
	"fmt"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	codeUnsupportedMedia = "unsupported_media_combination"
	maxReferenceImages   = 5 // LumixAI: start/reference images total ≤ 5 (end frame separate)
)

// videoJobRequest is the LumixAI POST /v1/videos/jobs JSON body. Images travel as data URIs
// because DramaClaw media URLs are internal and LumixAI cannot fetch them.
type videoJobRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt,omitempty"`
	AspectRatio      string   `json:"aspectRatio,omitempty"`
	Resolution       string   `json:"resolution,omitempty"`
	Duration         *int     `json:"duration,omitempty"`
	WithSoundEffects *bool    `json:"withSoundEffects,omitempty"`
	Image            string   `json:"image,omitempty"`
	Images           []string `json:"images,omitempty"`
	EndImage         string   `json:"endImage,omitempty"`
	MaxRefs          *int     `json:"maxRefs,omitempty"`
}

type unsupportedMediaError struct{ reason string }

func (e *unsupportedMediaError) Error() string { return codeUnsupportedMedia + ": " + e.reason }

// mediaFetcher turns a DC-Media image reference (http(s) or data URI) into a data URI.
type mediaFetcher func(rawURL string) (string, error)

func validateVideoMediaShape(req relaycommon.TaskSubmitReq) error {
	dc := relaycommon.DCMediaMetadata{}
	if err := req.UnmarshalMetadata(&dc); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}
	taskType, _ := req.Metadata["omni_reference_task_type"].(string)
	switch {
	case len(dc.ReferenceVideos) > 0, len(dc.ReferenceAudios) > 0:
		return &unsupportedMediaError{"LumixAI adapter does not accept reference video or audio yet"}
	case dc.ReferenceFile != "", dc.ReferenceLink != "":
		return &unsupportedMediaError{"LumixAI does not accept reference files or links"}
	case req.DurationAuto, taskType == "edit", taskType == "extend":
		return &unsupportedMediaError{"video edit and extend are not supported"}
	case len(dc.ReferenceImages) > maxReferenceImages:
		return &unsupportedMediaError{fmt.Sprintf("at most %d reference images", maxReferenceImages)}
	case strings.TrimSpace(req.Image) != "" && len(dc.ReferenceImages) > 0:
		return &unsupportedMediaError{"first frame cannot be combined with reference images"}
	case dc.LastFrameImage != "" && len(dc.ReferenceImages) > 0:
		return &unsupportedMediaError{"last frame cannot be combined with reference images"}
	case req.Duration > relaycommon.MaxTaskDurationSeconds:
		return &unsupportedMediaError{fmt.Sprintf("duration exceeds %d seconds", relaycommon.MaxTaskDurationSeconds)}
	}
	return nil
}

func buildVideoJob(req relaycommon.TaskSubmitReq, fetch mediaFetcher) (*videoJobRequest, error) {
	if err := validateVideoMediaShape(req); err != nil {
		return nil, err
	}
	dc := relaycommon.DCMediaMetadata{}
	if err := req.UnmarshalMetadata(&dc); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}
	job := &videoJobRequest{Model: req.Model, Prompt: req.Prompt, WithSoundEffects: dc.GenerateAudio}
	if ratio := strings.TrimSpace(dc.Ratio); ratio != "" && !strings.EqualFold(ratio, "auto") {
		job.AspectRatio = ratio
	}
	job.Resolution = strings.TrimSpace(dc.Resolution)
	if strings.HasSuffix(job.Resolution, "k") {
		job.Resolution = strings.ToUpper(job.Resolution) // LumixAI video tiers use 2K / 4K
	}
	if req.Duration > 0 {
		duration := req.Duration
		job.Duration = &duration
	}
	var err error
	if first := strings.TrimSpace(req.Image); first != "" {
		if job.Image, err = fetch(first); err != nil {
			return nil, fmt.Errorf("fetch first frame: %w", err)
		}
	}
	if last := strings.TrimSpace(dc.LastFrameImage); last != "" {
		if job.EndImage, err = fetch(last); err != nil {
			return nil, fmt.Errorf("fetch last frame: %w", err)
		}
	}
	for _, ref := range dc.ReferenceImages {
		dataURI, fetchErr := fetch(strings.TrimSpace(ref))
		if fetchErr != nil {
			return nil, fmt.Errorf("fetch reference image: %w", fetchErr)
		}
		job.Images = append(job.Images, dataURI)
	}
	if count := len(job.Images); count > 0 {
		job.MaxRefs = &count
	}
	return job, nil
}
