// Package plan compares the services the compose file asks for with the
// services that are running, and renders what a deploy would change.
//
// Owns the shared Service shape that compose and engine both reduce to,
// the Diff that produces one Change per service, and the text output
// (changes first, "no change" last, secret values never printed).
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
)

type Service struct {
	Name     string
	Image    string
	Digest   string
	ImageID  string
	TaggedID string
	Config   map[string]any
}

type Change struct {
	Service string
	Action  string
	Reason  []string
	Notes   []string
	Pull    bool
}

type Resolver func(ctx context.Context, ref string) (string, error)

func ShortDigest(digest string) string {
	if h, ok := strings.CutPrefix(digest, "sha256:"); ok && len(h) > 12 {
		return "sha256:" + h[:12] + "..."
	}
	return digest
}

func Diff(ctx context.Context, desired, running []Service, recorded map[string]map[string]any, resolve Resolver) []Change {
	var out []Change
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
		} else if desiredService.Config["build"] == nil && r.Digest != "" && resolve != nil {
			got, err := resolve(ctx, desiredService.Image)
			if err != nil {
				c.Notes = append(c.Notes, "could not check registry: "+err.Error())
			} else if got != r.Digest {
				c.Pull = true
				c.Reason = append(c.Reason, fmt.Sprintf("image %q moved: %s -> %s", desiredService.Image, ShortDigest(r.Digest), ShortDigest(got)))
			}
		}

		if old, ok := recorded[desiredService.Name]; ok {
			c.Reason = append(c.Reason, configChanges(old, desiredService.Config)...)
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
	return out
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

// image is compared against the running container above, with a better
// message, so the recorded-config diff leaves it out.
var checkedLive = map[string]bool{"image": true}

// configChanges lists every top-level setting that differs between the config
// recorded at the last deploy and the one rendered now.
func configChanges(old, cur map[string]any) []string {
	keys := map[string]bool{}
	for k := range old {
		keys[k] = true
	}
	for k := range cur {
		keys[k] = true
	}

	var out []string
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		if checkedLive[k] || reflect.DeepEqual(old[k], cur[k]) {
			continue
		}
		if k == "environment" { // values are hashes: name the keys, never print them
			names := changedEnv(old[k], cur[k])
			out = append(out, fmt.Sprintf("%d env value(s) changed (%s), values hidden", len(names), strings.Join(names, ", ")))
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s -> %s", k, compact(old[k]), compact(cur[k])))
	}
	return out
}

// compact prints a config value on one short line.
func compact(v any) string {
	if v == nil {
		return "none"
	}
	b, _ := json.Marshal(v)
	if s := string(b); len(s) <= 60 {
		return s
	}
	return string(b[:57]) + "..."
}

func changedEnv(old, cur any) []string {
	o, _ := old.(map[string]any)
	c, _ := cur.(map[string]any)
	var names []string
	for k := range o {
		if !reflect.DeepEqual(o[k], c[k]) {
			names = append(names, k)
		}
	}
	for k := range c {
		if _, ok := o[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}
