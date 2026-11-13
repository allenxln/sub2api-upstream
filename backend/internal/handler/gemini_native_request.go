package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type geminiNativeRequest struct {
	model              string
	action             string
	moderationProtocol string
}

type geminiNativeRequestParser func(*gin.Context, []byte) (geminiNativeRequest, bool)

func parseGeminiModelRequest(c *gin.Context, _ []byte) (geminiNativeRequest, bool) {
	model, action, err := parseGeminiModelAction(strings.TrimPrefix(c.Param("modelAction"), "/"))
	if err != nil {
		googleError(c, http.StatusNotFound, err.Error())
		return geminiNativeRequest{}, false
	}
	if !service.IsSafeGeminiModelPathSegment(model) {
		googleError(c, http.StatusBadRequest, "Invalid model in URL")
		return geminiNativeRequest{}, false
	}
	return geminiNativeRequest{model: model, action: action, moderationProtocol: service.ContentModerationProtocolGemini}, true
}

// GeminiV1BetaDecisions shares Gemini authentication, admission, account
// scheduling and accounting, while its parser and transport keep native JSON.
func (h *GatewayHandler) GeminiV1BetaDecisions(c *gin.Context) {
	h.handleGeminiV1BetaRequest(c, parseGeminiDecisionsRequest)
}

func parseGeminiDecisionsRequest(c *gin.Context, body []byte) (geminiNativeRequest, bool) {
	model, err := gemini.ValidateDecisionsRequest(body)
	if err != nil {
		googleError(c, http.StatusBadRequest, err.Error())
		return geminiNativeRequest{}, false
	}
	return geminiNativeRequest{model: model, action: gemini.DecisionsAction, moderationProtocol: service.ContentModerationProtocolGeminiDecisions}, true
}
