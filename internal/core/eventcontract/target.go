package eventcontract

import "strings"

type GatewaySelectionPolicy string

const (
	GatewaySelectionExplicitOnly             GatewaySelectionPolicy = "explicit_only"
	GatewaySelectionAllowSoleGatewayFallback GatewaySelectionPolicy = "allow_sole_gateway_fallback"
	GatewaySelectionAllowSurfaceDerived      GatewaySelectionPolicy = "allow_surface_derived_fallback"
)

type GatewayFailurePolicy string

const (
	GatewayFailureError      GatewayFailurePolicy = "error"
	GatewayFailureTypedError GatewayFailurePolicy = "typed_error"
	GatewayFailureNoop       GatewayFailurePolicy = "noop"
)

type TargetRef struct {
	GatewayID        string
	SurfaceSessionID string
	SelectionPolicy  GatewaySelectionPolicy
	FailurePolicy    GatewayFailurePolicy
}

func ExplicitTarget(gatewayID, surfaceSessionID string) TargetRef {
	return TargetRef{
		GatewayID:        strings.TrimSpace(gatewayID),
		SurfaceSessionID: strings.TrimSpace(surfaceSessionID),
		SelectionPolicy:  GatewaySelectionExplicitOnly,
		FailurePolicy:    GatewayFailureError,
	}
}

func (target TargetRef) Normalized() TargetRef {
	target.GatewayID = strings.TrimSpace(target.GatewayID)
	target.SurfaceSessionID = strings.TrimSpace(target.SurfaceSessionID)
	switch target.SelectionPolicy {
	case GatewaySelectionAllowSoleGatewayFallback, GatewaySelectionAllowSurfaceDerived:
	default:
		target.SelectionPolicy = GatewaySelectionExplicitOnly
	}
	switch target.FailurePolicy {
	case GatewayFailureTypedError, GatewayFailureNoop:
	default:
		target.FailurePolicy = GatewayFailureError
	}
	return target
}
