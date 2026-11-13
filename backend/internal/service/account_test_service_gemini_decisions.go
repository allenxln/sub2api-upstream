package service

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/gin-gonic/gin"
)

func createGeminiDecisionsTestPayload(model, prompt string) []byte {
	if strings.TrimSpace(prompt) == "" {
		prompt = "Hello, this is a synthetic connection test."
	}
	body, _ := json.Marshal(map[string]any{
		"model": model, "input": prompt,
		"questions": map[string]any{"q": map[string]string{
			"type": "noul", "instructions": "Does the input contain a greeting?",
		}},
	})
	return body
}

func (s *AccountTestService) processGeminiDecisionsResponse(c *gin.Context, reader io.Reader) error {
	body, err := readUpstreamResponseBodyLimited(reader, resolveUpstreamResponseReadLimit(s.cfg))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to read Decisions response")
	}
	if _, err := gemini.DecodeDecisionsResponse(body); err != nil {
		return s.sendErrorAndEnd(c, "Invalid Decisions response or usage")
	}
	s.sendEvent(c, TestEvent{Type: "content", Text: string(body)})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
