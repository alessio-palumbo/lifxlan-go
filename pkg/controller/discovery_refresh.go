package controller

import "errors"

// ErrDiscoveryRefreshUnsupported means a custom client does not expose refresh.
var ErrDiscoveryRefreshUnsupported = errors.New("client does not support discovery target refresh")

// RefreshDiscovery refreshes the existing client's broadcast target and sends
// discovery once. It never automatically switches interfaces, replaces sessions,
// or recreates sockets. Call after a known host subnet transition; failures can
// be retried once the selected interface is available. The Client interface is
// unchanged: custom clients may opt in with RefreshBroadcastTarget() error.
func (c *Controller) RefreshDiscovery() error {
	refresh, ok := c.client.(interface{ RefreshBroadcastTarget() error })
	if !ok {
		return ErrDiscoveryRefreshUnsupported
	}
	if err := refresh.RefreshBroadcastTarget(); err != nil {
		return err
	}
	return c.Discover()
}
