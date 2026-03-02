// Package hclfuncs provides functions to register on an hcl.EvalContext for
// use inside HCL expressions in collect job files.
//
// The three time functions here are written from scratch against the public
// cty API. They are not ports of Terraform's datetime funcs (which are
// BUSL-licensed in recent Terraform releases).
package hclfuncs

import (
	"fmt"
	"time"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// TimestampFunc returns the current UTC time formatted as RFC3339.
//
//	timestamp()  // "2026-04-11T09:15:04Z"
var TimestampFunc = function.New(&function.Spec{
	Params: []function.Parameter{},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		return cty.StringVal(time.Now().UTC().Format(time.RFC3339)), nil
	},
})

// TimeAddFunc adds a Go-style duration to an RFC3339 timestamp and returns
// the result as a new RFC3339 timestamp.
//
//	timeadd("2026-04-11T09:15:04Z", "1h30m")  // "2026-04-11T10:45:04Z"
var TimeAddFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "timestamp", Type: cty.String},
		{Name: "duration", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		ts, err := time.Parse(time.RFC3339, args[0].AsString())
		if err != nil {
			return cty.NilVal, fmt.Errorf("timeadd: invalid RFC3339 timestamp %q: %w", args[0].AsString(), err)
		}
		dur, err := time.ParseDuration(args[1].AsString())
		if err != nil {
			return cty.NilVal, fmt.Errorf("timeadd: invalid duration %q: %w", args[1].AsString(), err)
		}
		return cty.StringVal(ts.Add(dur).Format(time.RFC3339)), nil
	},
})

// FormatDateFunc formats an RFC3339 timestamp using a Go reference-time layout.
// Unlike Terraform's formatdate (which uses a bespoke YYYY-MM-DD layout
// language), this accepts Go's native reference time: "2006-01-02T15:04:05Z".
//
//	formatdate("2006-01-02", "2026-04-11T09:15:04Z")  // "2026-04-11"
//
// If you want the bespoke Terraform layout language, that belongs in a
// separate function.
var FormatDateFunc = function.New(&function.Spec{
	Params: []function.Parameter{
		{Name: "layout", Type: cty.String},
		{Name: "timestamp", Type: cty.String},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		layout := args[0].AsString()
		ts, err := time.Parse(time.RFC3339, args[1].AsString())
		if err != nil {
			return cty.NilVal, fmt.Errorf("formatdate: invalid RFC3339 timestamp %q: %w", args[1].AsString(), err)
		}
		return cty.StringVal(ts.Format(layout)), nil
	},
})

// Datetime returns the datetime function table ready to merge into an
// hcl.EvalContext.Functions map.
func Datetime() map[string]function.Function {
	return map[string]function.Function{
		"timestamp":  TimestampFunc,
		"timeadd":    TimeAddFunc,
		"formatdate": FormatDateFunc,
	}
}
