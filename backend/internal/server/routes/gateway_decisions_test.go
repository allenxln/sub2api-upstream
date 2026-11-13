package routes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type decisionsRouteAPIKeyRepo struct{ keyBillingRouteAPIKeyRepo }

func (*decisionsRouteAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

func TestGatewayDecisionsUsesGoogleAuthentication(t *testing.T) {
	router := newGatewayRoutesTestRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(`{"model":"gemini-decision-1","input":"sample","questions":{"q":{"type":"noul"}}}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
}

func TestGatewayDecisionsEnforcesBodyModelAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 42, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformGemini,
		SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-decision-1"}}}
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10, Concurrency: 1}
	apiKey := &service.APIKey{ID: 100, UserID: user.ID, User: user, GroupID: &group.ID, Group: group, Status: service.StatusActive, Key: "decisions-route-test-key"}
	cfg := &config.Config{RunMode: config.RunModeStandard, Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	keyService := service.NewAPIKeyService(&decisionsRouteAPIKeyRepo{keyBillingRouteAPIKeyRepo{apiKey: apiKey}}, nil, nil, nil, &keyBillingRouteRateRepo{}, nil, cfg)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil)},
		middleware.NewAPIKeyAuthMiddleware(keyService, nil, cfg), keyService, nil, nil, nil, nil, cfg)
	for _, tc := range []struct {
		name, header, key, body string
		status                  int
	}{
		{"bearer authentication", "Authorization", "Bearer decisions-route-test-key", `{"model":"gemini-decision-1","questions":{"q":{}}}`, 400},
		{"Google authentication", "x-goog-api-key", "decisions-route-test-key", `{"model":"gemini-decision-1","input":"sample","questions":{"q":{}},"stream":true}`, 400},
		{"forbidden body model", "x-goog-api-key", "decisions-route-test-key", `{"model":"private-model","input":"sample","questions":{"q":{}}}`, 404},
		{"duplicate model cannot bypass allowlist", "x-goog-api-key", "decisions-route-test-key", `{"model":"gemini-decision-1","Model":"private-model","input":"sample","questions":{"q":{}}}`, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(tc.body))
			request.Header.Set(tc.header, tc.key)
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
		})
	}
}

func TestGatewayDecisionsCompositeRouteKeepsPublicModelForAccounting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{{
		ID: 1, GroupID: 1, PublicModel: "decision-alias", MatchType: service.CompositeRouteMatchExact,
		TargetPlatform: service.PlatformGemini, UpstreamModel: "gemini-decision-1", Endpoint: service.CompositeRouteEndpointGemini,
		Priority: 1, Enabled: true,
	}}})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(1)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{
			ID: groupID, Platform: service.PlatformComposite,
			ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"decision-alias"}},
		}})
		c.Next()
	})
	router.Use(middleware.GroupModelAllowlist())
	router.Use(compositeGeminiTargetPlatformMiddleware(resolver))
	router.POST("/v1beta/decisions", compositeTargetPlatformMiddleware(resolver), func(c *gin.Context) {
		platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, service.PlatformGemini, platform)
		public, ok := service.RequestedPublicModelFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, "decision-alias", public)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"gemini-decision-1","input":[{"type":"text","text":"sample"}],"questions":{"q":{"type":"noul"}}}`, string(body))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(`{"model":"decision-alias","input":[{"type":"text","text":"sample"}],"questions":{"q":{"type":"noul"}}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}
