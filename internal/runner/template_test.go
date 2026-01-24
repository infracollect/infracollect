package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExpandVariables(t *testing.T) {
	fixedDate := time.Date(2026, 1, 24, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input string
		vars  TemplateVars
		want  string
	}{
		{
			name:  "no variables",
			input: "plain-text",
			vars:  TemplateVars{JobName: "test-job", Date: fixedDate},
			want:  "plain-text",
		},
		{
			name:  "job name only",
			input: "$JOB_NAME",
			vars:  TemplateVars{JobName: "my-job", Date: fixedDate},
			want:  "my-job",
		},
		{
			name:  "job name with prefix",
			input: "prefix-$JOB_NAME",
			vars:  TemplateVars{JobName: "test", Date: fixedDate},
			want:  "prefix-test",
		},
		{
			name:  "job name with suffix",
			input: "$JOB_NAME-suffix",
			vars:  TemplateVars{JobName: "test", Date: fixedDate},
			want:  "test-suffix",
		},
		{
			name:  "iso8601 date",
			input: "$JOB_DATE_ISO8601",
			vars:  TemplateVars{JobName: "test", Date: fixedDate},
			want:  "20260124T103000Z",
		},
		{
			name:  "rfc3339 date",
			input: "$JOB_DATE_RFC3339",
			vars:  TemplateVars{JobName: "test", Date: fixedDate},
			want:  "2026-01-24T10:30:00Z",
		},
		{
			name:  "combined variables",
			input: "$JOB_NAME-$JOB_DATE_ISO8601",
			vars:  TemplateVars{JobName: "infra-snapshot", Date: fixedDate},
			want:  "infra-snapshot-20260124T103000Z",
		},
		{
			name:  "all variables",
			input: "$JOB_NAME/$JOB_DATE_ISO8601/$JOB_DATE_RFC3339",
			vars:  TemplateVars{JobName: "job", Date: fixedDate},
			want:  "job/20260124T103000Z/2026-01-24T10:30:00Z",
		},
		{
			name:  "repeated variables",
			input: "$JOB_NAME-$JOB_NAME",
			vars:  TemplateVars{JobName: "dup", Date: fixedDate},
			want:  "dup-dup",
		},
		{
			name:  "empty job name",
			input: "prefix-$JOB_NAME-suffix",
			vars:  TemplateVars{JobName: "", Date: fixedDate},
			want:  "prefix--suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandVariables(tt.input, tt.vars)
			assert.Equal(t, tt.want, got)
		})
	}
}
