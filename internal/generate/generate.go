package generate

import (
	"context"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const pauseImgName = "registry.k8s.io/pause:3.7"

func Generate(ctx context.Context, imgName string, layerCount int, layerSize int64) error {
	ref, err := name.ParseReference(pauseImgName)
	if err != nil {
		return err
	}
	img, err := remote.Image(ref, remote.WithContext(ctx))
	if err != nil {
		return err
	}
	layers := []v1.Layer{}
	for range layerCount {
		layer, err := random.Layer(layerSize, types.OCILayer)
		if err != nil {
			return err
		}
		layers = append(layers, layer)
	}
	img, err = mutate.AppendLayers(img, layers...)
	if err != nil {
		return err
	}
	img, err = mutate.CreatedAt(img, v1.Time{Time: time.Now()})
	if err != nil {
		return err
	}
	tag, err := name.NewTag(imgName)
	if err != nil {
		return err
	}
	_, err = daemon.Write(tag, img, daemon.WithContext(ctx))
	if err != nil {
		return err
	}
	return nil
}
