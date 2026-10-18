//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCNYCatalogOwnsPricesAndRejectsUSDCacheAndFallback(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "model_pricing.json"), []byte(`{"deepseek-chat":{"input_cost_per_token":0.00000015,"output_cost_per_token":0.0000003}}`), 0600))
	file := filepath.Join(dir, "official-cny.json")
	body := `{"_meta":{"currency":"CNY"},"deepseek-chat":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000004,"cache_read_input_token_cost":0.00000002}}`
	require.NoError(t, os.WriteFile(file, []byte(body), 0600))
	cfg := &config.Config{Pricing: config.PricingConfig{Currency: "CNY", DataDir: dir, FallbackFile: file}}
	prices := NewPricingService(cfg, nil)
	require.NoError(t, prices.Initialize())
	defer prices.Stop()
	billing := NewBillingService(cfg, prices)
	p, err := billing.GetModelPricing("deepseek-chat")
	require.NoError(t, err)
	require.Equal(t, 0.000001, p.InputPricePerToken)
	require.Equal(t, 0.000004, p.OutputPricePerToken)
	require.Nil(t, prices.GetModelPricing("gpt-6-unlisted"))
	_, err = billing.GetModelPricing("claude-opus-5-5")
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	require.False(t, billing.HasIdentifiedTokenPricing("claude-opus-5-5"))
	require.Equal(t, "model_pricing.cny.json", filepath.Base(prices.getPricingFilePath()))
	require.Equal(t, "CNY", prices.GetStatus()["currency"])
	require.NoError(t, prices.reloadCustomPricingLayers())
	p, err = billing.GetModelPricing("deepseek-chat")
	require.NoError(t, err)
	require.Equal(t, 0.000001, p.InputPricePerToken)
}

func TestCNYCatalogRejectsUnlabelledPricesAndRemoteUSDSync(t *testing.T) {
	cfg := &config.Config{Pricing: config.PricingConfig{Currency: "CNY", DataDir: t.TempDir(), RemoteURL: "https://example.invalid/usd.json"}}
	require.Error(t, NewPricingService(cfg, nil).Initialize())
	cfg.Pricing.RemoteURL = ""
	prices := NewPricingService(cfg, nil)
	_, err := prices.parsePricingData([]byte(`{"gpt-5":{"input_cost_per_token":0.000001}}`))
	require.Error(t, err)
}

func TestCNYCatalogReusesNativeIntervalsAndHolidaySchedule(t *testing.T) {
	cfg := &config.Config{Pricing: config.PricingConfig{Currency: "CNY"}}
	prices := NewPricingService(cfg, nil)
	body := `{"_meta":{"currency":"CNY"},"deepseek-chat":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000004,"pricing":{"platform":"openai","models":["deepseek-chat"],"billing_mode":"token","input_price":0.000001,"output_price":0.000004,"time_pricing":{"timezone":"Asia/Shanghai","weekdays_only":true,"excluded_dates":["2026-10-01"],"periods":[{"start_time":"09:00:00","end_time":"12:00:00","multiplier":2}]},"intervals":[{"min_tokens":0,"max_tokens":32000,"input_price":0.000001,"output_price":0.000004},{"min_tokens":32000,"max_tokens":null,"input_price":0.000002,"output_price":0.000008,"sort_order":1}]}}}`
	var err error
	prices.pricingData, err = prices.parsePricingData([]byte(body))
	require.NoError(t, err)
	billing := NewBillingService(cfg, prices)
	resolver := NewModelPricingResolver(nil, billing)
	resolved := resolver.Resolve(t.Context(), PricingInput{Model: "deepseek-chat"})
	require.Len(t, resolved.Intervals, 2)
	require.NotNil(t, resolvedTimePricingSchedule(resolved))
	for _, tc := range []struct {
		at     string
		factor float64
	}{{"2026-09-30T10:00:00+08:00", 2}, {"2026-10-01T10:00:00+08:00", 1}} {
		at, err := time.Parse(time.RFC3339, tc.at)
		require.NoError(t, err)
		require.Equal(t, tc.factor, resolvedChannelTimeMultiplier(resolved, at))
	}
}

func TestPublishedCNYCatalogBillsPricesOnceWithNativeGroupRates(t *testing.T) {
	cfg := &config.Config{Pricing: config.PricingConfig{Currency: "CNY"}}
	prices := NewPricingService(cfg, nil)
	body, err := os.ReadFile("../../resources/model-pricing/model_prices_cny_20261017.json")
	require.NoError(t, err)
	prices.pricingData, err = prices.parsePricingData(body)
	require.NoError(t, err)
	require.Len(t, prices.pricingData, 276)
	billing := NewBillingService(cfg, prices)
	resolver := NewModelPricingResolver(nil, billing)
	for _, tc := range []struct {
		at   string
		want float64
	}{
		{"2026-09-30T10:00:00+08:00", 1},
		{"2026-10-01T10:00:00+08:00", .5},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		require.NoError(t, err)
		cost, err := billing.CalculateTokenCostForRequest(TokenCostRequest{
			Ctx: t.Context(), Model: "deepseek-flash", Tokens: UsageTokens{InputTokens: 1_000_000},
			RateMultiplier: .5, PricingAt: at, Resolver: resolver,
		})
		require.NoError(t, err)
		require.InDelta(t, tc.want, cost.ActualCost, 1e-10)
	}
}
