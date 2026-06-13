package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/infracollect/infracollect/internal/runner"
	"github.com/urfave/cli/v3"
)

var planCommand = &cli.Command{
	Name:      "plan",
	Usage:     "Show execution plan without running collectors or steps",
	Flags:     jobFlags(),
	Arguments: jobArguments(),
	Action: func(ctx context.Context, command *cli.Command) error {
		r, err := buildRunnerFromCommand(ctx, command)
		if err != nil {
			return err
		}

		plan, err := r.DryRun()
		if err != nil {
			return fmt.Errorf("failed to build execution plan: %w", err)
		}

		return printPlan(os.Stdout, plan)
	},
}

func printPlan(w io.Writer, plan *runner.Plan) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	fmt.Fprintf(tw, "Job:\t%s\n\n", plan.Job)

	for i, node := range plan.Nodes {
		seq := i + 1
		tag := fmt.Sprintf("[%s]", node.Kind)
		addr := fmt.Sprintf("%s/%s", node.Type, node.ID)

		switch {
		case node.ForEach && node.ForEachResolved:
			fmt.Fprintf(tw, "  %d.\t%s\t%s\t(for_each: %d → %s)\n",
				seq, tag, addr, len(node.ForEachKeys), strings.Join(node.ForEachKeys, ", "))
		case node.ForEach:
			fmt.Fprintf(tw, "  %d.\t%s\t%s\t(for_each: computed at runtime)\n", seq, tag, addr)
		default:
			fmt.Fprintf(tw, "  %d.\t%s\t%s\t\n", seq, tag, addr)
		}

		if node.Collector != "" {
			fmt.Fprintf(tw, "  \t\tcollector:\t%s\n", node.Collector)
		}
		if len(node.DependsOn) > 0 {
			fmt.Fprintf(tw, "  \t\tdepends on:\t%s\n", strings.Join(node.DependsOn, ", "))
		}
	}

	fmt.Fprintln(tw)
	fmt.Fprintf(tw, "Output:\n")
	fmt.Fprintf(tw, "  encoding:\t%s\n", plan.Output.Encoding)
	fmt.Fprintf(tw, "  sink:\t%s\n", plan.Output.Sink)
	if plan.Output.Archive != "" {
		fmt.Fprintf(tw, "  archive:\t%s\n", plan.Output.Archive)
	}
	if len(plan.Output.Steps) > 0 {
		fmt.Fprintf(tw, "  steps:\t%s\n", strings.Join(plan.Output.Steps, ", "))
	}

	return tw.Flush()
}
