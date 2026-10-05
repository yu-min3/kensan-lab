package deploymentobserver

import (
	"context"
	"errors"
	"fmt"
)

// APIProbe accepts only an ephemeral host-selected Pod whose current identity
// still equals the previously verified IP, image and runtime manifest.
func (k KubeCLI) APIProbe(ctx context.Context, p Plan) (UserPathState, error) {
	if err := p.validate(); err != nil {
		return UserPathState{}, err
	}
	selected := p.probePod
	if p.ProbeTransport != "kubernetes-api" || selected == nil || !safePodName(selected.Name) || selected.Namespace != namespace || selected.UID == "" || selected.AppLabel != "canary" || !p.ProbeIP.IsValid() || p.ProbeCIDR.IsValid() || selected.PodIP != p.ProbeIP || !selected.Ready {
		return UserPathState{}, errors.New("API proxy requires a host-verified resolved Pod")
	}
	api, err := k.privateAPI()
	if err != nil {
		return UserPathState{}, err
	}
	current, err := api.currentPod(ctx, selected.Name)
	if err != nil {
		return UserPathState{}, err
	}
	if current != *selected {
		return UserPathState{}, errors.New("selected Pod changed before API proxy")
	}

	route := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:8000/proxy/api/release", namespace, selected.Name)
	body, err := api.get(ctx, route, 4<<10)
	if err != nil {
		return UserPathState{}, err
	}
	release, err := parseReleaseResponse(body)
	if err != nil || release != p.ExpectedRelease {
		return UserPathState{}, errors.New("API proxy release marker did not match")
	}
	return UserPathState{StatusCode: 200, Path: p.ProbePath, ObservedRelease: release}, nil
}
