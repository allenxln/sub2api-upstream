package gemini

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
)

func TestValidateDecisionsRequest(t *testing.T) {
	for _, body := range []string{
		`{"model":"decision-alias","input":"sample","questions":{"q":{"type":"noul","instructions":"Is this valid?"}}}`,
		`{"model":"gemini-decision-1","input":[{"type":"text","text":"sample"},{"type":"image","data":"AA==","mime_type":"image/png"}],"questions":{"q":{"type":"choice","criteria":{"a":"A","b":"B"}}},"provider_extension":{"kept":true}}`,
	} {
		if model, err := ValidateDecisionsRequest([]byte(body)); err != nil || model == "" {
			t.Fatalf("valid native envelope rejected: model=%q err=%v", model, err)
		}
	}
	for _, body := range []string{
		`[]`,
		`{"input":"sample","questions":{"q":{}}}`,
		`{"model":"gemini-decision-1","input":null,"questions":{"q":{}}}`,
		`{"model":"gemini-decision-1","input":"sample","questions":{}}`,
		`{"model":"gemini-decision-1","input":"sample","questions":{"q":{}},"stream":true}`,
		`{"model":"public","model":"private","input":"sample","questions":{"q":{}}}`,
		`{"model":"public","Model":"private","input":"sample","questions":{"q":{}}}`,
	} {
		if _, err := ValidateDecisionsRequest([]byte(body)); err == nil {
			t.Errorf("unsafe or incomplete envelope accepted: %s", body)
		}
	}
}

func TestDecisionsQuestionLimit(t *testing.T) {
	questions := make(map[string]any)
	for i := 0; i < 1001; i++ {
		questions["q"+strconv.Itoa(i)] = map[string]string{"type": "noul", "instructions": "Is this true?"}
	}
	body, _ := json.Marshal(map[string]any{"model": "gemini-decision-1", "input": "sample", "questions": questions})
	if _, err := ValidateDecisionsRequest(body); err == nil {
		t.Fatal("1001 questions must be rejected before upstream work")
	}
	delete(questions, "q1000")
	body, _ = json.Marshal(map[string]any{"model": "gemini-decision-1", "input": "sample", "questions": questions})
	if _, err := ValidateDecisionsRequest(body); err != nil {
		t.Fatalf("1000 questions rejected: %v", err)
	}
}

func TestNewDecisionsRequestPreservesMultimodalInputAndMapsOnlyModel(t *testing.T) {
	body := []byte(`{"model":"public-decision","input":[{"type":"text","text":"a document"},{"type":"image","data":"AA==","mime_type":"image/png"}],"questions":{"q":{"type":"noul","instructions":"Is there text?"}},"provider_extension":{"kept":true}}`)
	for _, base := range []string{"https://provider.test", "https://provider.test/v1beta/"} {
		req, err := NewDecisionsRequest(context.Background(), base, "synthetic-key", "gemini-decision-1", body)
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.URL.String() != "https://provider.test/v1beta/decisions" {
			t.Fatalf("wrong native endpoint: %s %s", req.Method, req.URL)
		}
		if req.Header.Get("x-goog-api-key") != "synthetic-key" || req.Header.Get("Authorization") != "" || req.URL.RawQuery != "" {
			t.Fatal("API key must be sent in its native header only")
		}
		got, _ := io.ReadAll(req.Body)
		var original, mapped map[string]any
		_ = json.Unmarshal(body, &original)
		_ = json.Unmarshal(got, &mapped)
		original["model"] = "gemini-decision-1"
		want, _ := json.Marshal(original)
		actual, _ := json.Marshal(mapped)
		if string(actual) != string(want) {
			t.Fatalf("payload changed outside model mapping: %s", got)
		}
	}
}

func TestDecodeDecisionsResponseUsesAuthoritativeZeroOutputUsage(t *testing.T) {
	body := []byte(`{"answers":{"damaged":{"type":"noul","noul":0.99,"confidence":0.99}},"usage":{"total_input_tokens":174,"total_output_tokens":0,"total_tokens":174,"input_tokens_by_modality":[{"modality":"text","tokens":174}]},"provider_extension":{"kept":true}}`)
	result, err := DecodeDecisionsResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage.InputTokens != 174 || result.Usage.OutputTokens != 0 {
		t.Fatalf("wrong absolute token counts: %+v", result.Usage)
	}
	if string(result.Body) != string(body) {
		t.Fatal("native probabilities, modality usage and extensions must pass through unchanged")
	}
}

func TestDecodeDecisionsResponseRejectsUnbillableOrWrongProtocolResponses(t *testing.T) {
	for _, body := range []string{
		`{"answers":{"q":{}},"usage":{}}`,
		`{"answers":{"q":{}},"usage":{"total_input_tokens":-1}}`,
		`{"answers":{"q":{}},"usage":{"total_input_tokens":1.5}}`,
		`{"answers":{"q":{}},"usage":{"total_input_tokens":174,"total_output_tokens":2}}`,
		`{"answers":{"q":{}},"usage":{"total_input_tokens":174,"total_output_tokens":0,"total_tokens":175}}`,
		`{"usage":{"total_input_tokens":174,"total_output_tokens":0}}`,
		`{"error":{"message":"not an answer"}}`,
	} {
		if _, err := DecodeDecisionsResponse([]byte(body)); err == nil {
			t.Errorf("invalid accounting/protocol accepted: %s", body)
		}
	}
}
