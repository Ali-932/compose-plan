// Package plan compares the services the compose file asks for with the
// services that are running, and renders what a deploy would change.
//
// Owns the shared Service shape that compose and engine both reduce to,
// the Diff that produces one Change per service, and the text output
// (changes first, "no change" last, secret values never printed).
package plan

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

type Service struct {
	Name     string
	Image    string
	Digest   string
	ImageID  string
	TaggedID string
	MemLimit int64
	Env      map[string]string
	Build    bool
}

type Change struct {
	Service string
	Action  string
	Reason  []string
	Notes   []string
	Pull    bool
}

type Resolver func(ctx context.Context, ref string) (string, error)

func BytesToHumanSize(b int64) string {
	if b == 0 {
		return "unlimited"
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}

	units := []string{"B", "k", "M", "G", "T", "P", "E"}
	size := float64(b)
	exp := 0

	for size >= 1024 && exp < len(units)-1 {
		size /= 1024
		exp++
	}

	return fmt.Sprintf("%.1f %sB", size, units[exp])
}

func ShortDigest(digest string) string {
	if h, ok := strings.CutPrefix(digest, "sha256:"); ok && len(h) > 12 {
		return "sha256:" + h[:12] + "..."
	}
	return digest
}

func Diff(ctx context.Context, desired, running []Service, resolve Resolver) ([]Change, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	var out []Change
	defer cancel()
	live := map[string]Service{}
	for _, svc := range running {
		live[svc.Name] = svc
	}

	for _, desiredService := range desired {
		r, ok := live[desiredService.Name]
		if !ok {
			out = append(out, Change{Service: desiredService.Name, Action: "create", Reason: []string{"not running"}})
			continue
		}

		c := Change{Service: desiredService.Name, Action: "no change"}

		if desiredService.Image != r.Image {
			c.Reason = append(c.Reason, fmt.Sprintf("image %s -> %s", r.Image, desiredService.Image))
		} else if r.TaggedID != "" && r.TaggedID != r.ImageID {
			c.Reason = append(c.Reason, fmt.Sprintf("image %q changed on this machine %s -> %s", desiredService.Image, ShortDigest(r.ImageID), ShortDigest(r.TaggedID)))
		} else if !desiredService.Build && r.Digest != "" && resolve != nil {
			got, err := resolve(ctx, desiredService.Image)
			if err != nil {
				c.Notes = append(c.Notes, "could not check registry: "+err.Error())
			} else if got != r.Digest {
				c.Pull = true
				c.Reason = append(c.Reason, fmt.Sprintf("image %q moved: %s -> %s", desiredService.Image, ShortDigest(r.Digest), ShortDigest(got)))
			}
		}

		if r.MemLimit != desiredService.MemLimit {
			c.Reason = append(c.Reason, fmt.Sprintf("Different Memory Limit %s running vs %s stated", BytesToHumanSize(r.MemLimit), BytesToHumanSize(desiredService.MemLimit)))
		}
		var changedKeys []string
		for k, v := range desiredService.Env {
			if r.Env[k] != v {
				changedKeys = append(changedKeys, k)
			}
		}
		if len(changedKeys) > 0 {
			sort.Strings(changedKeys)
			c.Reason = append(c.Reason, fmt.Sprintf("%d env value(s) changed (%s), values hidden", len(changedKeys), strings.Join(changedKeys, ", ")))
		}

		if len(c.Reason) > 0 {
			c.Action = "update"
		}
		out = append(out, c)
		delete(live, desiredService.Name)

	}
	for name := range live {
		out = append(out, Change{Service: name, Action: "remove", Reason: []string{"not in compose file"}})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ni := out[i].Action == "no change"
		nj := out[j].Action == "no change"
		if ni == true && nj == false {
			return false
		} else if ni == false && nj == true {
			return true
		}
		return out[i].Service < out[j].Service
	})
	return out, nil
}

func Render(w io.Writer, changes []Change) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, c := range changes {
		lines := append(c.Reason, c.Notes...)
		if len(lines) == 0 {
			lines = []string{""}
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Service, c.Action, lines[0])
		for _, l := range lines[1:] {
			fmt.Fprintf(tw, "\t\t%s\n", l)
		}
	}

	tw.Flush()
}

func (s Service) Fingerprint() string {
	if s.Digest != "" {
		return s.Digest
	}
	return s.ImageID
}
