package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
)

type geminiNativeActionKey struct{}

// WithGeminiNativeAction lets account eligibility enforce the operation before
// acquiring or binding an account. The transport rechecks the complete account.
func WithGeminiNativeAction(ctx context.Context, action string) context.Context {
	return context.WithValue(ctx, geminiNativeActionKey{}, action)
}

func isGeminiDecisionsAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformGemini && account.Type == AccountTypeAPIKey
}

// The native transport owns proxy selection, retries and account-error policy.
// This response adapter only preserves Decisions JSON and translates its usage.
func (s *GeminiMessagesCompatService) handleNativeDecisionsResponse(c *gin.Context, resp *http.Response, model, upstreamModel, requestID string, started time.Time, account *Account) (*ForwardResult, error) {
	body, err := readUpstreamResponseBodyLimited(resp.Body, resolveUpstreamResponseReadLimit(s.cfg))
	if err == nil {
		var decoded *gemini.DecisionsResponse
		decoded, err = gemini.DecodeDecisionsResponse(body)
		if err == nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
			c.Data(resp.StatusCode, "application/json", decoded.Body)
			return &ForwardResult{
				RequestID: requestID, UpstreamHeaders: resp.Header.Clone(),
				Model: model, UpstreamModel: upstreamModel, UpstreamResponseModel: decoded.Model,
				Usage:    ClaudeUsage{InputTokens: decoded.Usage.InputTokens},
				Duration: time.Since(started),
			}, nil
		}
	}
	// The provider may have consumed input even when its response is malformed.
	// Preserve an operations trail without logging the prompt, answer or key.
	setOpsUpstreamError(c, resp.StatusCode, err.Error(), "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
		UpstreamStatusCode: resp.StatusCode, UpstreamRequestID: requestID,
		Kind: "response_error", Message: err.Error(),
	})
	return nil, s.writeGoogleError(c, http.StatusBadGateway, "Invalid Decisions response or usage")
}
