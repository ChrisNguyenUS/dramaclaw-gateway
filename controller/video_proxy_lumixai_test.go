package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Manna: LumixAI result files need X-API-Key; the proxy must send it to the channel origin.
func TestVideoProxyLumixAISendsAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "lumix-test-key", r.Header.Get("X-API-Key"))
		assert.Equal(t, "/v1/videos/results/j1/download", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("MP4DATA"))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))

	baseURL := upstream.URL
	channel := &model.Channel{Id: 7, Type: constant.ChannelTypeLumixAI, Key: "lumix-test-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(channel).Error)
	task := &model.Task{TaskID: "task_lumix", UserId: 1, ChannelId: 7, Status: model.TaskStatusSuccess}
	task.PrivateData.UpstreamResultURL = upstream.URL + "/v1/videos/results/j1/download"
	require.NoError(t, db.Create(task).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_lumix/content", nil)
	c.Params = gin.Params{{Key: "task_id", Value: "task_lumix"}}
	c.Set("id", 1)

	VideoProxy(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "MP4DATA", recorder.Body.String())
}
