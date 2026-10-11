package controller

import (
	"errors"
	"testing"
)

type refreshClient struct {
	*mockClient
	refreshes int
	err       error
}

func (c *refreshClient) RefreshBroadcastTarget() error { c.refreshes++; return c.err }

func TestExplicitDiscoveryRefresh(t *testing.T) {
	c := &refreshClient{mockClient: newMockClient()}
	ctrl := &Controller{client: c}
	if err := ctrl.RefreshDiscovery(); err != nil || c.refreshes != 1 || len(c.broadcasts) != 1 {
		t.Fatal(err)
	}
	c.err = errors.New("interface unavailable")
	if err := ctrl.RefreshDiscovery(); !errors.Is(err, c.err) || len(c.broadcasts) != 1 {
		t.Fatal(err)
	}
	ctrl.client = newMockClient()
	if err := ctrl.RefreshDiscovery(); !errors.Is(err, ErrDiscoveryRefreshUnsupported) {
		t.Fatal(err)
	}
}
