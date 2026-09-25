package cli

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/Ali-932/compose-plan/internal/registry"
)

// RollBack deploys the images recorded in history entry seq again.
func RollBack(ctx context.Context, opts Options, seq int) error {
	project, err := compose.Load(ctx, opts.Files, opts.EnvFiles, opts.Name)
	if err != nil {
		return err
	}
	entries, err := history.Read(historyPath(project))
	if err != nil {
		return err
	}
	entry, err := history.Find(entries, seq)
	if err != nil {
		return err
	}
	services, err := engine.Running(ctx, project.Name)
	if err != nil {
		return err
	}

	running := make(map[string]string)
	for _, svc := range services {
		running[svc.Name] = svc.Fingerprint()
	}
	imageOf := make(map[string]string)
	for _, svc := range project.Services {
		imageOf[svc.Name] = svc.Image
	}

	needChange := make(map[string]string)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, svc := range slices.Sorted(maps.Keys(entry.Services)) {
		digest := entry.Services[svc]
		if _, inFile := imageOf[svc]; !inFile {
			fmt.Fprintf(tw, "%s\tskipped\tin entry #%d but not in the compose file.\n", svc, seq)
			continue
		}
		if cur, ok := running[svc]; !ok {
			fmt.Fprintf(tw, "%s\tcreate\t%s\n", svc, plan.ShortDigest(digest))
			needChange[svc] = digest
		} else if cur != digest {
			fmt.Fprintf(tw, "%s\timage\t%s -> %s\n", svc, plan.ShortDigest(cur), plan.ShortDigest(digest))
			needChange[svc] = digest
		}
	}
	tw.Flush()

	if len(needChange) == 0 {
		fmt.Printf("entry #%d is already what is running\n", seq)
		return nil
	}

	tmpFile, err := os.CreateTemp("", "compose-plan-rollback-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	var composeYAML strings.Builder
	composeYAML.WriteString("services:\n")
	for _, svc := range slices.Sorted(maps.Keys(needChange)) {
		digest := needChange[svc]
		fullDigest := imageOf[svc] + "@" + digest
		if !engine.HasImage(ctx, digest) {
			if _, err := registry.Resolve(ctx, fullDigest); err != nil {
				return fmt.Errorf("%s: image from entry #%d no longer exists on this machine or the registry, cannot roll back", svc, seq)
			}
		}
		composeYAML.WriteString(fmt.Sprintf("  %s:\n    image: %s\n", svc, fullDigest))
	}
	if _, err := tmpFile.WriteString(composeYAML.String()); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	withOverride := opts
	withOverride.Files = append(slices.Clone(project.Files), tmpFile.Name())
	if err := RunApply(ctx, withOverride, fmt.Sprintf("rollback to #%d", seq)); err != nil {
		return err
	}
	fmt.Println("containers were restored, data was not: database migrations run since that entry are still applied")
	if head := gitHead(project.WorkingDir); entry.Commit != "" && entry.Commit != head {
		fmt.Printf("config at #%d was commit %s, current tree is %s; to restore it: git checkout %s && compose-plan apply\n", seq, entry.Commit, head, entry.Commit)
	}
	return nil
}
