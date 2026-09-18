// Package engine reads what is running from the Docker daemon through
// github.com/moby/moby/client (the docker/docker module is frozen at v28).
// Compose labels containers with com.docker.compose.project, .service and
// .config-hash; those are how a container maps back to a service.
package engine

import (
	"context"
	"strings"

	"github.com/Ali-932/compose-plan/internal/plan"
)
import "github.com/moby/moby/client"

func Running(ctx context.Context, projectName string) ([]plan.Service, error) {
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()
	result, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: true,
		Filters: make(client.Filters).
			Add("label", "com.docker.compose.project="+projectName),
	})
	if err != nil {
		return nil, err
	}
	var out []plan.Service
	seen := map[string]bool{}
	for _, c := range result.Items {
		name := c.Labels["com.docker.compose.service"]
		if seen[name] {
			continue
		}
		seen[name] = true
		insp, err := apiClient.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			return nil, err
		}

		s := plan.Service{
			Name:     name,
			Image:    insp.Container.Config.Image,
			MemLimit: insp.Container.HostConfig.Memory,
			Env:      map[string]string{},
		}
		for _, kv := range insp.Container.Config.Env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				s.Env[k] = v
			}

		}
		img, err := apiClient.ImageInspect(ctx, c.ImageID)

		if err == nil && len(img.RepoDigests) > 0 {
			_, s.Digest, _ = strings.Cut(img.RepoDigests[0], "@")

		}
		s.ImageID = c.ImageID
		if tagged, err := apiClient.ImageInspect(ctx, s.Image); err == nil {
			s.TaggedID = tagged.ID
		}
		out = append(out, s)
	}
	return out, nil

}
