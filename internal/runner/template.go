package runner

import (
	"strings"
	"time"

	"github.com/infracollect/infracollect/internal/engine"
)

// TemplateVars contains variables that can be expanded in template strings.
type TemplateVars struct {
	JobName string
	Date    time.Time
}

// ExpandVariables replaces template variables in the input string.
// Supported variables:
//   - $JOB_NAME: The job's metadata.name
//   - $JOB_DATE_ISO8601: Current UTC time in ISO8601 basic format (20060102T150405Z)
//   - $JOB_DATE_RFC3339: Current UTC time in RFC3339 format (2006-01-02T15:04:05Z)
func ExpandVariables(input string, vars TemplateVars) string {
	result := input
	result = strings.ReplaceAll(result, "$JOB_NAME", vars.JobName)
	result = strings.ReplaceAll(result, "$JOB_DATE_ISO8601", vars.Date.Format(engine.ISO8601Basic))
	result = strings.ReplaceAll(result, "$JOB_DATE_RFC3339", vars.Date.Format(time.RFC3339))
	return result
}
