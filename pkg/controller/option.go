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
