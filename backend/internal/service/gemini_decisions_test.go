package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type decisionsTestTransport struct {
	client *http.Client
	proxy  string
	calls  int
}

func (u *decisionsTestTransport) Do(req *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
	u.proxy = proxy
	u.calls++
	return u.client.Do(req)
}

func (u *decisionsTestTransport) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func TestGeminiDecisionsNativePreservesProtocolProxyAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"public-decision","input":[{"type":"text","text":"inspect this image"},{"type":"image","data":"AA==","mime_type":"image/png"}],"questions":{"q":{"type":"noul","instructions":"Is there text?"}},"provider_extension":{"kept":true}}`)
	response := `{"answers":{"q":{"type":"noul","noul":0.99,"confidence":0.99}},"usage":{"total_input_tokens":174,"total_output_tokens":0,"total_tokens":174},"provider_extension":{"kept":true}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/decisions", r.URL.Path)
		require.Equal(t, "synthetic-google-key", r.Header.Get("x-goog-api-key"))
		require.Empty(t, r.Header.Get("Authorization"))
		got, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, strings.Replace(string(body), "public-decision", "gemini-decision-1", 1), string(got))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-goog-request-id", "decision-request")
		_, _ = io.WriteString(w, response)
	}))
	defer server.Close()
	upstream := &decisionsTestTransport{client: server.Client()}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true}}}, httpUpstream: upstream}
	proxyID := int64(31)
	account := &Account{ID: 20, Platform: PlatformGemini, Type: AccountTypeAPIKey, ProxyID: &proxyID,
		Proxy:       &Proxy{Protocol: "http", Host: "egress.test", Port: 18081, Username: "user", Password: "pass"},
		Credentials: map[string]any{"base_url": server.URL, "api_key": "synthetic-google-key", "model_mapping": map[string]any{"public-decision": "gemini-decision-1"}}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", strings.NewReader(string(body)))
	result, err := svc.ForwardNative(context.Background(), c, account, "public-decision", "decisions", false, body)
	require.NoError(t, err)
	require.Equal(t, response, recorder.Body.String())
	require.Equal(t, "http://user:pass@egress.test:18081", upstream.proxy)
	require.Equal(t, 1, upstream.calls)
	require.Equal(t, "public-decision", result.Model)
	require.Equal(t, "gemini-decision-1", result.UpstreamModel)
	require.Equal(t, "decision-request", result.RequestID)
	require.Equal(t, 174, result.Usage.InputTokens)
	require.Zero(t, result.Usage.OutputTokens)
	require.Zero(t, result.Usage.CacheReadInputTokens)
	require.False(t, result.ImageEndpoint)
	require.Empty(t, result.ReasoningEffort)
}

func TestGeminiDecisionsRejectsStreamingBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", nil)
	result, err := svc.ForwardNative(context.Background(), c, &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}, "gemini-decision-1", "decisions", true, []byte(`{"input":"sample"}`))
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGeminiDecisionsUsesNativeCallerErrorAndFailoverPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{400, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := &systemOneHTTPUpstream{do: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"synthetic upstream error"}}`))}, nil
			}}
			cfg := &config.Config{}
			svc := &GeminiMessagesCompatService{cfg: cfg, httpUpstream: upstream}
			account := &Account{ID: 21, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic-key"}}
			repo := &systemOnePolicyAccountRepo{account: account}
			svc.accountRepo = repo
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/decisions", nil)
			result, err := svc.ForwardNative(context.Background(), c, account, "gemini-decision-1", "decisions", false, []byte(`{"model":"gemini-decision-1","input":"sample","questions":{"q":{"type":"noul","instructions":"Is this valid?"}}}`))
			require.Nil(t, result)
			require.Error(t, err)
			var failover *UpstreamFailoverError
			if status == 400 {
				require.False(t, errors.As(err, &failover))
				require.Equal(t, 400, recorder.Code)
				require.Zero(t, repo.errorCalls)
				require.Zero(t, repo.rateLimitedCalls)
			} else {
				require.ErrorAs(t, err, &failover)
				require.Equal(t, status, failover.StatusCode)
				require.Empty(t, recorder.Body.String())
				if status == 429 {
					require.Positive(t, repo.rateLimitedCalls)
				}
			}
		})
	}
}

func TestGeminiDecisionsRejectsGenerationEndpointBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"answers":{"q":{"type":"noul","noul":0.99}},"usage":{"total_input_tokens":174,"total_output_tokens":0}}`)
	}))
	defer server.Close()
	upstream := &decisionsTestTransport{client: server.Client()}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true}}}, httpUpstream: upstream}
	account := &Account{ID: 23, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": server.URL, "api_key": "synthetic-key"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-decision-1:generateContent", nil)
	result, err := svc.ForwardNative(context.Background(), c, account, "gemini-decision-1", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"sample"}]}]}`))
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, upstream.calls)
}

func TestGeminiDecisionsAdminConnectionUsesDeclaredModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1beta/decisions", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"model":"gemini-decision-1"`)
		require.Contains(t, string(body), `"questions"`)
		_, _ = io.WriteString(w, `{"answers":{"q":{"type":"noul","noul":0.99}},"usage":{"total_input_tokens":174,"total_output_tokens":0}}`)
	}))
	defer server.Close()
	upstream := &decisionsTestTransport{client: server.Client()}
	svc := &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true}}}, httpUpstream: upstream}
	account := &Account{ID: 24, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": server.URL, "api_key": "synthetic-key", "model_mapping": map[string]any{"public-decision": "gemini-decision-1"}}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/accounts/24/test", nil)
	require.NoError(t, svc.testGeminiAccountConnection(c, account, "", "Hello, this is a synthetic test."))
	require.Equal(t, 1, upstream.calls)
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}
