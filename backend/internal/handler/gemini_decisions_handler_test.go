package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiDecisionsRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(`{}`))
	(&GatewayHandler{}).GeminiV1BetaDecisions(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestGeminiDecisionsRejectsInvalidRequestsBeforeScheduling(t *testing.T) {
	for _, body := range []string{
		`{"model":"gemini-decision-1","input":"sample","questions":{"q":{"type":"noul"}},"stream":true}`,
		`{"model":"gemini-decision-1","questions":{"q":{"type":"noul"}}}`,
		`{"model":"public","model":"private","input":"sample","questions":{"q":{"type":"noul"}}}`,
	} {
		gin.SetMode(gin.TestMode)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(body))
		groupID := int64(1)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 2, UserID: 3, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGemini}})
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3, Concurrency: 1})
		(&GatewayHandler{}).GeminiV1BetaDecisions(c)
		require.Equal(t, http.StatusBadRequest, recorder.Code, body)
	}
}

func TestGeminiDecisionsEndpointMetadata(t *testing.T) {
	require.Equal(t, "/v1beta/decisions", NormalizeInboundEndpoint("/google/v1beta/decisions"))
	require.Equal(t, "/v1beta/decisions", DeriveUpstreamEndpoint("/v1beta/decisions", "/v1beta/decisions", service.PlatformGemini))
}

func TestGeminiDecisionsPromptGuardPrecedesScheduling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := blockingHandlerPromptEngine()
	h := &GatewayHandler{gatewayService: &service.GatewayService{}, securityAuditCoordinator: securityaudit.NewCoordinator(nil, engine)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"gemini-decision-1","input":"synthetic input","questions":{"q":{"type":"noul"}},"generationConfig":{"responseModalities":["IMAGE"]}}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(body))
	groupID := int64(1)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 2, UserID: 3, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGemini}})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3, Concurrency: 1})
	// Scheduling/billing dependencies are intentionally absent: a blocked request must stop here.
	require.NotPanics(t, func() { h.GeminiV1BetaDecisions(c) })
	require.Equal(t, http.StatusForbidden, recorder.Code)
	evaluated, _, requests := engine.snapshot()
	require.Equal(t, 1, evaluated)
	require.Len(t, requests, 1)
	require.Equal(t, service.ContentModerationProtocolGeminiDecisions, requests[0].Protocol)
}
