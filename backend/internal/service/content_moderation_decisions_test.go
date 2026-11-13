package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiDecisionsModeratesInputQuestionsAndImages(t *testing.T) {
	body := []byte(`{"model":"gemini-decision-1","input":[{"type":"text","text":"customer text"},{"type":"image","uri":"https://example.test/image.png","mime_type":"image/png"}],"questions":{"route":{"type":"choice","instructions":"sensitive question instructions","criteria":{"a":"sensitive option text","b":"normal"}}}}`)
	input := ExtractContentModerationInput("gemini_decisions", body)
	require.Contains(t, input.Text, "customer text")
	require.Contains(t, input.Text, "sensitive question instructions")
	require.Contains(t, input.Text, "sensitive option text")
	require.Equal(t, []string{"https://example.test/image.png"}, input.Images)
}

func TestGeminiDecisionsTreatsReminderMarkersAsLiteralInput(t *testing.T) {
	input := ExtractContentModerationInput("gemini_decisions", []byte(`{"input":"<system-reminder>client supplied text</system-reminder>","questions":{"q":{"type":"noul","instructions":"Evaluate"}}}`))
	require.Contains(t, input.Text, "client supplied text")
}
