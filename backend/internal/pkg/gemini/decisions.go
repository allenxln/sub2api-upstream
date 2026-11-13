package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/sjson"
)

const (
	DecisionsPath        = "/v1beta/decisions"
	DecisionsAction      = "decisions"
	GeminiDecision1Model = "gemini-decision-1"
)

// ValidateDecisionsRequest validates the routing envelope. Google remains the
// owner of question/media schemas and token limits; unknown fields pass through.
func ValidateDecisionsRequest(body []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return "", errors.New("Decisions request must be a JSON object")
	}
	fields := make(map[string]json.RawMessage)
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return "", errors.New("Invalid Decisions request")
		}
		name, ok := token.(string)
		if !ok || seen[strings.ToLower(name)] {
			return "", errors.New("Duplicate Decisions request field")
		}
		seen[strings.ToLower(name)] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return "", errors.New("Invalid Decisions request field")
		}
		fields[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return "", errors.New("Invalid Decisions request")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", errors.New("Unexpected data after Decisions request")
	}
	var model string
	if json.Unmarshal(fields["model"], &model) != nil || strings.TrimSpace(model) == "" {
		return "", errors.New("Decisions model must be a non-empty string")
	}
	if input := bytes.TrimSpace(fields["input"]); len(input) == 0 || bytes.Equal(input, []byte("null")) {
		return "", errors.New("Decisions input is required")
	}
	var questions map[string]json.RawMessage
	if json.Unmarshal(fields["questions"], &questions) != nil || len(questions) < 1 || len(questions) > 1000 {
		return "", errors.New("Decisions requires between 1 and 1000 questions")
	}
	if stream, exists := fields["stream"]; exists && !bytes.Equal(bytes.TrimSpace(stream), []byte("false")) {
		return "", errors.New("Decisions does not support streaming")
	}
	return strings.TrimSpace(model), nil
}

// NewDecisionsRequest maps only the body model and uses Google's native API-key
// header. URL authorization and account/proxy selection belong to the service.
func NewDecisionsRequest(ctx context.Context, baseURL, apiKey, model string, body []byte) (*http.Request, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	base = strings.TrimSuffix(base, "/v1beta")
	if base == "" || strings.TrimSpace(apiKey) == "" || strings.TrimSpace(model) == "" {
		return nil, errors.New("Decisions upstream, API key and model are required")
	}
	mapped, err := sjson.SetBytes(body, "model", model)
	if err != nil {
		return nil, errors.New("Invalid Decisions request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+DecisionsPath, bytes.NewReader(mapped))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", apiKey)
	return request, nil
}

type DecisionsUsage struct {
	InputTokens  int
	OutputTokens int
}

type DecisionsResponse struct {
	Body  []byte
	Model string
	Usage DecisionsUsage
}

// DecodeDecisionsResponse uses authoritative usage rather than estimating from
// the answers. A generative response must never be accepted as a decision.
func DecodeDecisionsResponse(body []byte) (*DecisionsResponse, error) {
	var envelope struct {
		Model   string                     `json:"model"`
		Answers map[string]json.RawMessage `json:"answers"`
		Usage   *struct {
			Input  *int `json:"total_input_tokens"`
			Output int  `json:"total_output_tokens"`
			Total  *int `json:"total_tokens"`
		} `json:"usage"`
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Answers) == 0 || envelope.Usage == nil ||
		envelope.Usage.Input == nil || *envelope.Usage.Input < 0 || envelope.Usage.Output != 0 ||
		(len(envelope.Error) > 0 && !bytes.Equal(bytes.TrimSpace(envelope.Error), []byte("null"))) {
		return nil, errors.New("Invalid Decisions response or usage")
	}
	if envelope.Usage.Total != nil && *envelope.Usage.Total != *envelope.Usage.Input {
		return nil, errors.New("Inconsistent Decisions token usage")
	}
	return &DecisionsResponse{
		Body: body, Model: envelope.Model,
		Usage: DecisionsUsage{InputTokens: *envelope.Usage.Input, OutputTokens: 0},
	}, nil
}
