package adapters

import (
	"context"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
)

// RunEffects runs effects in order using a renderer configured from d.
func RunEffects(ctx context.Context, d device.Device, send SendFunc, runs ...effects.RunConfig) error {
	return effects.RunSequence(ctx, NewRendererForDevice(d, send), runs...)
}
