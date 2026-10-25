package lumixai

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	baseURL string
	apiKey  string
}

type jobResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   []struct {
		DownloadURL string `json:"download_url"`
	} `json:"data"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	if a.baseURL == "" {
		a.baseURL = constant.ChannelBaseURLs[constant.ChannelTypeLumixAI]
	}
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := validateVideoMediaShape(req); err != nil {
		var unsupported *unsupportedMediaError
		if errors.As(err, &unsupported) {
			return service.TaskErrorWrapperLocal(err, codeUnsupportedMedia, http.StatusBadRequest)
		}
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/videos/jobs", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	if info.IsModelMapped {
		req.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = req.Model
	}
	job, err := buildVideoJob(req, fetchAsDataURI)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(job)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// fetchAsDataURI downloads an image through the gateway (which can reach DramaClaw's internal
// media URLs) and returns it as a data URI. Data URIs pass through unchanged.
func fetchAsDataURI(rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "data:") {
		return rawURL, nil
	}
	mimeType, data, err := service.GetImageFromUrl(rawURL)
	if err != nil {
		return "", err
	}
	return "data:" + mimeType + ";base64," + data, nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	resp, err := channel.DoTaskApiRequest(a, c, info, requestBody)
	if resp != nil {
		normalizeAccepted(resp)
	}
	return resp, err
}

// normalizeAccepted maps LumixAI's 202 Accepted to 200, because the task relay treats any
// non-200 submit response as a failure (relay/relay_task.go).
func normalizeAccepted(resp *http.Response) {
	if resp.StatusCode == http.StatusAccepted {
		resp.StatusCode = http.StatusOK
	}
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var job jobResponse
	if err := common.Unmarshal(body, &job); err != nil || job.ID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("lumixai returned no job id: %s", body), "invalid_response", http.StatusBadGateway)
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.CreatedAt = time.Now().Unix()
	video.Model = info.OriginModelName
	c.JSON(http.StatusOK, video)
	return job.ID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	if baseURL = strings.TrimRight(baseURL, "/"); baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeLumixAI]
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/videos/jobs/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var job jobResponse
	if err := common.Unmarshal(respBody, &job); err != nil {
		return nil, fmt.Errorf("unmarshal lumixai job failed: %w", err)
	}
	result := &relaycommon.TaskInfo{TaskID: job.ID}
	switch job.Status {
	case "pending":
		result.Status, result.Progress = model.TaskStatusQueued, "10%"
	case "running":
		result.Status, result.Progress = model.TaskStatusInProgress, "50%"
	case "done":
		result.Progress = "100%"
		if len(job.Data) == 0 || strings.TrimSpace(job.Data[0].DownloadURL) == "" {
			result.Status, result.Reason = model.TaskStatusFailure, "empty_result: LumixAI finished without a result file"
			return result, nil
		}
		result.Status = model.TaskStatusSuccess
		result.RemoteUrl = a.absoluteURL(job.Data[0].DownloadURL)
	case "error":
		result.Status, result.Progress = model.TaskStatusFailure, "100%"
		result.Reason = job.Error
	default:
		result.Status, result.Progress = model.TaskStatusInProgress, "30%"
	}
	return result, nil
}

func (a *TaskAdaptor) absoluteURL(downloadURL string) string {
	if strings.HasPrefix(downloadURL, "http://") || strings.HasPrefix(downloadURL, "https://") {
		return downloadURL
	}
	return a.baseURL + "/" + strings.TrimLeft(downloadURL, "/")
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	video := dto.NewOpenAIVideo()
	video.ID = originTask.TaskID
	video.TaskID = originTask.TaskID
	video.Status = originTask.Status.ToVideoStatus()
	video.SetProgressStr(originTask.Progress)
	video.CreatedAt = originTask.CreatedAt
	video.CompletedAt = originTask.UpdatedAt
	video.Model = originTask.Properties.OriginModelName
	if originTask.PrivateData.UpstreamResultURL != "" {
		proxyURL := taskcommon.BuildPublicProxyURL(originTask.TaskID)
		video.SetMetadata("url", proxyURL)
		video.SetMetadata("video_url", proxyURL)
	}
	if originTask.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Message: originTask.FailReason, Code: "lumixai_task_failed"}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string    { return ModelList }
func (a *TaskAdaptor) GetChannelName() string    { return ChannelName }
func (a *TaskAdaptor) GetCapabilities() []string { return []string{"video"} }
