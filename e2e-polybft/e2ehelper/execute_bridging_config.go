package e2ehelper

import "time"

type executeBridgingConfig struct {
	waitForUnexpectedBridges bool
	stopValidatorAfter       time.Duration
	stopValidatorIndexes     []int
	startValidatorAfter      time.Duration
	startValidatorIndexes    []int
}

func newExecuteBridgingConfig(opts ...ExecuteBridgingOption) *executeBridgingConfig {
	config := &executeBridgingConfig{}

	for _, x := range opts {
		x(config)
	}

	return config
}

type ExecuteBridgingOption func(config *executeBridgingConfig)

func WithWaitForUnexpectedBridges(waitForUnexpectedBridges bool) ExecuteBridgingOption {
	return func(config *executeBridgingConfig) {
		config.waitForUnexpectedBridges = waitForUnexpectedBridges
	}
}

func WithValidatorStopConfig(stopValidatorAfter time.Duration, stopValidatorIndexes []int) ExecuteBridgingOption {
	return func(config *executeBridgingConfig) {
		config.stopValidatorAfter = stopValidatorAfter
		config.stopValidatorIndexes = stopValidatorIndexes
	}
}

func WithValidatorStartConfig(startValidatorAfter time.Duration, startValidatorIndexes []int) ExecuteBridgingOption {
	return func(config *executeBridgingConfig) {
		config.startValidatorAfter = startValidatorAfter
		config.startValidatorIndexes = startValidatorIndexes
	}
}
