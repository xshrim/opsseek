package client

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	containerd "github.com/containerd/containerd/v2/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// OpenContainerd connects directly to the containerd gRPC API. Docker API
// hosts are deliberately not accepted here.
func OpenContainerd(input ConnectionInput) (*containerd.Client, error) {
	if err := ValidateDaemonType(input.DaemonType); err != nil {
		return nil, err
	}
	if input.DaemonType != "" && !strings.EqualFold(input.DaemonType, "containerd") {
		return nil, fmt.Errorf("container daemon is %q, not containerd", input.DaemonType)
	}
	raw := strings.TrimSpace(input.DockerHost)
	if raw == "" {
		raw = "unix:///run/containerd/containerd.sock"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return nil, fmt.Errorf("invalid containerd endpoint")
	}
	var target string
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		var d net.Dialer
		if input.TimeoutSeconds != nil {
			d.Timeout = TimeoutSeconds(input.TimeoutSeconds)
		}
		switch strings.ToLower(u.Scheme) {
		case "unix":
			if u.Host != "" || u.Path == "" {
				return nil, fmt.Errorf("invalid containerd Unix socket URL")
			}
			return d.DialContext(ctx, "unix", u.Path)
		case "tcp", "grpc", "grpcs", "https":
			if u.Host == "" || u.Path != "" && u.Path != "/" {
				return nil, fmt.Errorf("invalid containerd gRPC endpoint")
			}
			return d.DialContext(ctx, "tcp", u.Host)
		default:
			return nil, fmt.Errorf("unsupported containerd endpoint scheme %q", u.Scheme)
		}
	}
	var creds credentials.TransportCredentials = insecure.NewCredentials()
	if strings.EqualFold(u.Scheme, "grpcs") || strings.EqualFold(u.Scheme, "https") || input.DockerCA != "" || input.DockerCert != "" || input.DockerKey != "" || input.DockerSkipVerify {
		cfg := ConnectionInput{DockerHost: "https://" + u.Host, DockerCA: input.DockerCA, DockerCert: input.DockerCert, DockerKey: input.DockerKey, DockerSkipVerify: input.DockerSkipVerify}
		resolved, err := resolveConnection(cfg, false)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := TLSConfig(resolved)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(tlsConfig)
	}
	target = "passthrough:///containerd"
	dialCtx, cancel := context.WithTimeout(context.Background(), TimeoutSeconds(input.TimeoutSeconds))
	defer cancel()
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(creds), grpc.WithContextDialer(dialer), grpc.WithBlock()}
	if u.Host != "" {
		dialOptions = append(dialOptions, grpc.WithAuthority(u.Host))
	}
	conn, err := grpc.DialContext(dialCtx, target, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("connect to containerd gRPC endpoint: %w", err)
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		namespace = "k8s.io"
	}
	cli, err := containerd.NewWithConn(conn, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return cli, nil
}
