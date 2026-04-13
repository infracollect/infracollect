package hclfuncs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestTimestampFunc(t *testing.T) {
	before := time.Now().UTC()
	result, err := TimestampFunc.Call([]cty.Value{})
	require.NoError(t, err)

	ts, parseErr := time.Parse(time.RFC3339, result.AsString())
	require.NoError(t, parseErr, "result should be valid RFC3339")

	after := time.Now().UTC()
	assert.False(t, ts.Before(before.Truncate(time.Second)), "timestamp should be >= before")
	assert.False(t, ts.After(after.Add(time.Second)), "timestamp should be <= after")
}

func TestTimeAddFunc(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		duration  string
		expected  string
		wantErr   string
	}{
		{
			name:      "add one hour",
			timestamp: "2026-04-11T09:15:04Z",
			duration:  "1h",
			expected:  "2026-04-11T10:15:04Z",
		},
		{
			name:      "add complex duration",
			timestamp: "2026-04-11T09:15:04Z",
			duration:  "1h30m",
			expected:  "2026-04-11T10:45:04Z",
		},
		{
			name:      "subtract duration",
			timestamp: "2026-04-11T09:15:04Z",
			duration:  "-2h",
			expected:  "2026-04-11T07:15:04Z",
		},
		{
			name:      "invalid timestamp",
			timestamp: "not-a-timestamp",
			duration:  "1h",
			wantErr:   "invalid RFC3339 timestamp",
		},
		{
			name:      "invalid duration",
			timestamp: "2026-04-11T09:15:04Z",
			duration:  "not-a-duration",
			wantErr:   "invalid duration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TimeAddFunc.Call([]cty.Value{
				cty.StringVal(tt.timestamp),
				cty.StringVal(tt.duration),
			})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.AsString())
		})
	}
}

func TestFormatDateFunc(t *testing.T) {
	tests := []struct {
		name      string
		layout    string
		timestamp string
		expected  string
		wantErr   string
	}{
		{
			name:      "date only",
			layout:    "2006-01-02",
			timestamp: "2026-04-11T09:15:04Z",
			expected:  "2026-04-11",
		},
		{
			name:      "ISO8601 basic",
			layout:    "20060102T150405Z",
			timestamp: "2026-04-11T09:15:04Z",
			expected:  "20260411T091504Z",
		},
		{
			name:      "human readable",
			layout:    "Mon, 02 Jan 2006",
			timestamp: "2026-04-11T09:15:04Z",
			expected:  "Sat, 11 Apr 2026",
		},
		{
			name:      "invalid timestamp",
			layout:    "2006-01-02",
			timestamp: "bad",
			wantErr:   "invalid RFC3339 timestamp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatDateFunc.Call([]cty.Value{
				cty.StringVal(tt.layout),
				cty.StringVal(tt.timestamp),
			})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.AsString())
		})
	}
}

func TestDatetime_ReturnsAllFunctions(t *testing.T) {
	funcs := Datetime()
	assert.Contains(t, funcs, "timestamp")
	assert.Contains(t, funcs, "timeadd")
	assert.Contains(t, funcs, "formatdate")
	assert.Len(t, funcs, 3)
}
