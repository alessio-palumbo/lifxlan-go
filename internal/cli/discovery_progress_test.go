package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestDiscoverySpinnerClearsOnCompletion(t *testing.T) {
	var out bytes.Buffer
	if err := waitForDiscovery(context.Background(), time.Nanosecond, &out, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Discovering devices") || !strings.HasSuffix(out.String(), "\r\x1b[2K") {
		t.Fatal(out.String())
	}
}

func TestDiscoverySpinnerClearsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelWriter{limit: 1, cancel: cancel}
	err := waitForDiscovery(ctx, time.Hour, out, true)
	if !errors.Is(err, context.Canceled) || !strings.HasSuffix(out.String(), "\r\x1b[2K") {
		t.Fatalf("err=%v output=%q", err, out.String())
	}
}

func TestDiscoveryWithoutAnimationNeverWrites(t *testing.T) {
	if err := waitForDiscovery(context.Background(), time.Nanosecond, brokenWriter{}, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := waitForDiscovery(ctx, time.Hour, &out, true); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, out.String())
	}
}

func TestDiscoverySpinnerOutputFailure(t *testing.T) {
	if err := waitForDiscovery(context.Background(), time.Hour, brokenWriter{}, true); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("got %v", err)
	}
}
