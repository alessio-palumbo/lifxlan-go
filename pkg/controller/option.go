package controller

import "time"

// Option overrides configurable Controller's options.
type Option func(*Controller) error

// WithClient sets the Controller's client to the given interface.
func WithClient(c Client) Option {
	return func(ctrl *Controller) error {
		ctrl.client = c
		return nil
	}
}

// WithDiscoveryPeriod sets the discovery period to the given duration.
func WithDiscoveryPeriod(d time.Duration) Option {
	return func(ctrl *Controller) error {
		ctrl.cfg.discoveryPeriod = d
		return nil
	}
}

// WithHFStateRefreshPeriod sets the high frequency state refresh period to the given duration.
func WithHFStateRefreshPeriod(d time.Duration) Option {
	return func(ctrl *Controller) error {
		ctrl.cfg.highFrequencyStateRefreshPeriod = d
		return nil
	}
}

// WithLFStateRefreshPeriod sets the low frequency state refresh period to the given duration.
func WithLFStateRefreshPeriod(d time.Duration) Option {
	return func(ctrl *Controller) error {
		ctrl.cfg.lowFrequencyStateRefreshPeriod = d
		return nil
	}
}

// WithPreflightHandshakeTimeout sets the timeout after which the initial handshake is stopped
// and normal tickets start pulling state periodically.
// For busy network setting a longer timeout ensures all devices receive the initial messages
// before start polling for state. For normal network the default should do.
func WithPreflightHandshakeTimeout(d time.Duration) Option {
	return func(ctrl *Controller) error {
		ctrl.cfg.preflightHandshakeTimeout = d
		return nil
	}
}
