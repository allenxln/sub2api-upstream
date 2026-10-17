//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelTimePricingExcludedDates(t *testing.T) {
	cfg := &ChannelTimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true,
		ExcludedDates: []string{"2026-10-01"},
		Periods:       []ChannelTimePricingPeriod{{StartTime: "09:00:00", EndTime: "12:00:00", Multiplier: 2}}}
	for _, tc := range []struct {
		at   string
		want float64
	}{
		{"2026-09-30T01:00:00Z", 2},
		{"2026-10-01T01:00:00Z", 1},
		{"2026-10-02T01:00:00Z", 2},
		{"2026-10-03T01:00:00Z", 1},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		require.NoError(t, err)
		require.Equal(t, tc.want, cfg.MultiplierAt(at), tc.at)
	}
	copy := ChannelModelPricing{TimePricing: cfg}.Clone()
	copy.TimePricing.ExcludedDates[0] = "2026-10-02"
	require.Equal(t, "2026-10-01", cfg.ExcludedDates[0])
}

func TestChannelTimePricingRejectsInvalidExcludedDates(t *testing.T) {
	for _, date := range []string{"2026-02-30", "2026-1-01", "2026-10-01T00:00:00Z", ""} {
		cfg := &ChannelTimePricing{Timezone: "Asia/Shanghai", ExcludedDates: []string{date},
			Periods: []ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}}}
		require.Error(t, validateChannelTimePricing(cfg), date)
	}
}

func TestResolvedTimePricingRetainsExcludedProbeDate(t *testing.T) {
	cfg := &ChannelTimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, ExcludedDates: []string{"2026-01-05"}, Periods: []ChannelTimePricingPeriod{{StartTime: "09:00:00", EndTime: "12:00:00", Multiplier: 2}}}
	resolved := &ResolvedPricing{Source: PricingSourceChannel, channelPricing: &ChannelModelPricing{TimePricing: cfg}}
	schedule := resolvedTimePricingSchedule(resolved)
	require.NotNil(t, schedule)
	require.Len(t, schedule.Periods, 1)
	require.Equal(t, cfg.ExcludedDates, schedule.ExcludedDates)
}
