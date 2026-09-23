// Package registry asks a registry what a tag points to right now, through
// github.com/google/go-containerregistry with the credentials Docker already
// stores. Any failure means "could not check", never a guess.
package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func Resolve(ctx context.Context, image string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", err
	}
	img, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		var terr *transport.Error
		if errors.As(err, &terr) && (terr.StatusCode == http.StatusNotFound || terr.StatusCode == http.StatusUnauthorized) {
			return "", fmt.Errorf("%s not found on %s, assuming a local image", image, ref.Context().RegistryStr())
		}
		return "", err
	}
	return img.Digest.String(), nil
}
