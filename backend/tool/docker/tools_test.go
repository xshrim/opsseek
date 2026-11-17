package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

func TestValidateContainerPath(t *testing.T) {
	for _, path := range []string{"", "relative.log", "/var/log/../etc/passwd", "/"} {
		if _, err := validateContainerPath(path); err == nil {
			t.Errorf("validateContainerPath(%q) succeeded", path)
		}
	}
	if got, err := validateContainerPath("/var/log/app.log"); err != nil || got != "/var/log/app.log" {
		t.Fatalf("validateContainerPath = %q, %v", got, err)
	}
}

func TestResolveContainerIdentifierRejectsInternalDatabaseID(t *testing.T) {
	if _, err := resolveContainerIdentifier("251683f4-d022-4153-886a-2eec581bf110", ""); err == nil {
		t.Fatal("database UUID must not be sent to Docker")
	}
	if got, err := resolveContainerIdentifier("", "orders-api"); err != nil || got != "orders-api" {
		t.Fatalf("external container name = %q, %v", got, err)
	}
}

func TestContainerListenersPrefersPublishedPortAndFallsBackToNetworkAddress(t *testing.T) {
	listeners := containerListeners(container.InspectResponse{NetworkSettings: &container.NetworkSettings{
		NetworkSettingsBase: container.NetworkSettingsBase{Ports: nat.PortMap{
			nat.Port("8080/tcp"): {{HostIP: "127.0.0.1", HostPort: "18080"}},
			nat.Port("9090/tcp"): nil,
		}},
		Networks: map[string]*network.EndpointSettings{
			"default": {IPAddress: "172.20.0.8"},
		},
	}})
	if len(listeners) != 2 || listeners[0] != (ContainerListener{Address: "127.0.0.1", Port: 18080, Protocol: "tcp"}) || listeners[1] != (ContainerListener{Address: "172.20.0.8", Port: 9090, Protocol: "tcp"}) {
		t.Fatalf("listeners = %#v", listeners)
	}
}
