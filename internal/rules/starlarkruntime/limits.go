package starlarkruntime

import (
	"fmt"
	"math"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	// ManifestPath is the fixed location of package metadata inside a bundle.
	ManifestPath = "ruleset.json"

	defaultMaxBundleBytes     = 8 << 20
	defaultMaxExpandedBytes   = 8 << 20
	defaultMaxSourceFileBytes = 1 << 20
	defaultMaxFiles           = 64
	defaultMaxExecutionSteps  = 100_000
	defaultMaxValueDepth      = 32
	defaultMaxValueNodes      = 16_384
	defaultMaxCallBytes       = 4 << 20
	defaultMaxCachedBundles   = 64
)

// Limits bounds archive parsing, cached programs, interpreter work, and the
// JSON-neutral values exchanged with Starlark. Zero fields select safe
// defaults; custom values may only reduce them and negative signed fields are
// invalid.
type Limits struct {
	MaxBundleBytes     int64
	MaxExpandedBytes   int64
	MaxSourceFileBytes int64
	MaxFiles           int
	MaxExecutionSteps  uint64
	MaxValueDepth      int
	MaxValueNodes      int
	MaxCollectionItems int
	MaxCallBytes       int
	MaxCachedBundles   int
}

// DefaultLimits returns the production defaults used by NewLoader.
func DefaultLimits() Limits {
	return Limits{
		MaxBundleBytes:     defaultMaxBundleBytes,
		MaxExpandedBytes:   defaultMaxExpandedBytes,
		MaxSourceFileBytes: defaultMaxSourceFileBytes,
		MaxFiles:           defaultMaxFiles,
		MaxExecutionSteps:  defaultMaxExecutionSteps,
		MaxValueDepth:      defaultMaxValueDepth,
		MaxValueNodes:      defaultMaxValueNodes,
		MaxCollectionItems: core.MaxCollectionItems,
		MaxCallBytes:       defaultMaxCallBytes,
		MaxCachedBundles:   defaultMaxCachedBundles,
	}
}

func normalizeLimits(limits Limits) (Limits, error) {
	defaults := DefaultLimits()
	if limits.MaxBundleBytes == 0 {
		limits.MaxBundleBytes = defaults.MaxBundleBytes
	}
	if limits.MaxExpandedBytes == 0 {
		limits.MaxExpandedBytes = defaults.MaxExpandedBytes
	}
	if limits.MaxSourceFileBytes == 0 {
		limits.MaxSourceFileBytes = defaults.MaxSourceFileBytes
	}
	if limits.MaxFiles == 0 {
		limits.MaxFiles = defaults.MaxFiles
	}
	if limits.MaxExecutionSteps == 0 {
		limits.MaxExecutionSteps = defaults.MaxExecutionSteps
	}
	if limits.MaxValueDepth == 0 {
		limits.MaxValueDepth = defaults.MaxValueDepth
	}
	if limits.MaxValueNodes == 0 {
		limits.MaxValueNodes = defaults.MaxValueNodes
	}
	if limits.MaxCollectionItems == 0 {
		limits.MaxCollectionItems = defaults.MaxCollectionItems
	}
	if limits.MaxCallBytes == 0 {
		limits.MaxCallBytes = defaults.MaxCallBytes
	}
	if limits.MaxCachedBundles == 0 {
		limits.MaxCachedBundles = defaults.MaxCachedBundles
	}

	if limits.MaxBundleBytes < 0 || limits.MaxExpandedBytes < 0 || limits.MaxSourceFileBytes < 0 ||
		limits.MaxFiles < 0 || limits.MaxValueDepth < 0 || limits.MaxValueNodes < 0 ||
		limits.MaxCollectionItems < 0 || limits.MaxCallBytes < 0 || limits.MaxCachedBundles < 0 {
		return Limits{}, fmt.Errorf("%w: limits must not be negative", ErrInvalidBundle)
	}
	if limits.MaxSourceFileBytes > limits.MaxExpandedBytes {
		return Limits{}, fmt.Errorf("%w: source file limit exceeds expanded bundle limit", ErrInvalidBundle)
	}
	if limits.MaxBundleBytes == math.MaxInt64 || limits.MaxExpandedBytes == math.MaxInt64 {
		return Limits{}, fmt.Errorf("%w: byte limits must leave room for overflow detection", ErrInvalidBundle)
	}
	if limits.MaxBundleBytes > defaults.MaxBundleBytes ||
		limits.MaxExpandedBytes > defaults.MaxExpandedBytes ||
		limits.MaxSourceFileBytes > defaults.MaxSourceFileBytes ||
		limits.MaxFiles > defaults.MaxFiles ||
		limits.MaxExecutionSteps > defaults.MaxExecutionSteps ||
		limits.MaxValueDepth > defaults.MaxValueDepth ||
		limits.MaxValueNodes > defaults.MaxValueNodes ||
		limits.MaxCollectionItems > defaults.MaxCollectionItems ||
		limits.MaxCallBytes > defaults.MaxCallBytes ||
		limits.MaxCachedBundles > defaults.MaxCachedBundles {
		return Limits{}, fmt.Errorf("%w: custom limits may only reduce defaults", ErrInvalidBundle)
	}
	return limits, nil
}
