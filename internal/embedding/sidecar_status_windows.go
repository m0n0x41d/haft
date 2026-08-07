//go:build windows

package embedding

import (
	"context"
	"errors"
)

func LoadSidecarStatus(_ context.Context, _ SidecarStatusOptions) (SidecarStatusReport, error) {
	return SidecarStatusReport{
		SharedEnabled: false,
		Warnings:      []string{"shared embedding sidecar status is unsupported on windows"},
	}, errors.New("shared embedding sidecar status is unsupported on windows")
}
