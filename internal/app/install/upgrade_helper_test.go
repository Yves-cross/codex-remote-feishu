package install

import "testing"

func TestGatewayRecovered(t *testing.T) {
	tests := []struct {
		name  string
		state upgradeHelperBootstrapState
		want  bool
	}{
		{name: "no gateways", state: upgradeHelperBootstrapState{}, want: true},
		{
			name: "connected gateway",
			state: upgradeHelperBootstrapState{Gateways: []struct {
				State string `json:"state"`
			}{{State: "connected"}}},
			want: true,
		},
		{
			name: "disabled only",
			state: upgradeHelperBootstrapState{Gateways: []struct {
				State string `json:"state"`
			}{{State: "disabled"}}},
			want: false,
		},
		{
			name: "degraded gateway",
			state: upgradeHelperBootstrapState{Gateways: []struct {
				State string `json:"state"`
			}{{State: "degraded"}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gatewayRecovered(tt.state); got != tt.want {
				t.Fatalf("gatewayRecovered() = %t, want %t", got, tt.want)
			}
		})
	}
}
