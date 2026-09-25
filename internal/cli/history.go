package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/history"
	"github.com/Ali-932/compose-plan/internal/plan"
)

func RunHistory(ctx context.Context, opts Options) error {
	project, err := compose.Load(ctx, opts.Files, opts.EnvFiles, opts.Name)
	if err != nil {
		return err
	}
	entries, err := history.Read(historyPath(project))
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no deploys recorded yet")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%s\n", e.Seq, e.Time.Format("Jan _2 15:04"), e.User, e.Commit, e.Summary)
	}
	return tw.Flush()
}

func RunDiff(ctx context.Context, opts Options, a, b int) error {
	project, err := compose.Load(ctx, opts.Files, opts.EnvFiles, opts.Name)
	if err != nil {
		return err
	}
	entries, err := history.Read(historyPath(project))
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no deploys recorded yet")
		return nil
	}
	aEntry, err := history.Find(entries, a)
	if err != nil {
		return err
	}
	bEntry, err := history.Find(entries, b)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for svc, digest := range aEntry.Services {
		if bDigest, ok := bEntry.Services[svc]; !ok {
			fmt.Fprintf(tw, "%s\tremoved\t%s\n", svc, plan.ShortDigest(digest))
		} else if digest != bDigest {
			fmt.Fprintf(tw, "%s\timage\t%s -> %s\n", svc, plan.ShortDigest(digest), plan.ShortDigest(bDigest))
		}
	}
	for svc, digest := range bEntry.Services {
		if _, ok := aEntry.Services[svc]; !ok {
			fmt.Fprintf(tw, "%s\tadded\t%s\n", svc, plan.ShortDigest(digest))
		}
	}
	return tw.Flush()
}
