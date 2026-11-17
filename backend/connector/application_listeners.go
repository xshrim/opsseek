package connector

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"opsseek/backend/engine"
	kclient "opsseek/backend/mcpserver/kubernetes/client"
	dockertool "opsseek/backend/tool/docker"
	host "opsseek/backend/tool/host"
	kt "opsseek/backend/tool/kubernetes"
)

// ResolveApplicationListeners derives the current addressable endpoints for
// one configured runtime binding. It deliberately leaves Agent resources to
// their remote MCP execution path, which cannot be called from this local
// application read without a user-scoped engine run.
func (s *Service) ResolveApplicationListeners(ctx context.Context, runtimeKind, resourceID string, selector map[string]any) ([]string, error) {
	switch strings.TrimSpace(runtimeKind) {
	case "virtual_machine":
		return s.hostApplicationListeners(ctx, resourceID, selector)
	case "containerized":
		return s.dockerApplicationListeners(ctx, resourceID, selector)
	case "cloud_native":
		return s.kubernetesApplicationListeners(ctx, resourceID, selector)
	default:
		return nil, fmt.Errorf("unsupported application runtime %q", runtimeKind)
	}
}

func (s *Service) hostApplicationListeners(ctx context.Context, resourceID string, selector map[string]any) ([]string, error) {
	item, err := s.runtimeResource(ctx, resourceID, "Host")
	if err != nil {
		return nil, err
	}
	connection, err := s.hostConnection(ctx, item)
	if err != nil {
		return nil, err
	}
	commandLine := selectorValue(selector, "process_command_line")
	processes, err := host.Processes(ctx, host.ProcessesInput{ConnectionInput: connection, CommandLine: commandLine, Limit: 2})
	if err != nil {
		return nil, err
	}
	if processes.MatchCount != 1 || len(processes.Processes) != 1 {
		return nil, fmt.Errorf("configured process locator matched %d processes", processes.MatchCount)
	}
	listeners, err := host.Listeners(ctx, host.ListenersInput{ConnectionInput: connection, PID: processes.Processes[0].PID})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(listeners.Listeners))
	for _, listener := range listeners.Listeners {
		result = appendAddress(result, connection.Host, listener.Port)
	}
	return uniqueAddresses(result), nil
}

func (s *Service) dockerApplicationListeners(ctx context.Context, resourceID string, selector map[string]any) ([]string, error) {
	item, err := s.runtimeResource(ctx, resourceID, "Docker")
	if err != nil {
		return nil, err
	}
	connection, err := s.dockerConnection(ctx, item)
	if err != nil {
		return nil, err
	}
	listeners, err := dockertool.ContainerListeners(ctx, dockertool.ContainerListenersInput{
		ConnectionInput: connection,
		ContainerName:   selectorValue(selector, "container_name"),
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(listeners.Listeners))
	for _, listener := range listeners.Listeners {
		result = appendAddress(result, listener.Address, listener.Port)
	}
	return uniqueAddresses(result), nil
}

func (s *Service) kubernetesApplicationListeners(ctx context.Context, resourceID string, selector map[string]any) ([]string, error) {
	connection, err := s.kubernetesRuntimeConnection(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	pods, err := kt.WorkloadPods(ctx, connection, selectorValue(selector, "workload_kind"), selectorValue(selector, "namespace"), selectorValue(selector, "workload_name"), 50)
	if err != nil {
		return nil, err
	}
	podName := selectorValue(selector, "pod_name")
	result := make([]string, 0)
	for _, pod := range pods.Pods {
		if podName != "" && pod.Name != podName {
			continue
		}
		for _, port := range pod.Ports {
			result = appendAddress(result, pod.IP, int(port.Port))
		}
	}
	return uniqueAddresses(result), nil
}

func (s *Service) runtimeResource(ctx context.Context, resourceID, kind string) (engine.ContextResource, error) {
	if s.resources == nil {
		return engine.ContextResource{}, fmt.Errorf("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, strings.TrimSpace(resourceID))
	if err != nil {
		return engine.ContextResource{}, err
	}
	if item.Kind != kind {
		return engine.ContextResource{}, fmt.Errorf("resource %q is not a %s resource", item.Name, kind)
	}
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return engine.ContextResource{}, fmt.Errorf("listener resolution for %s agent resources is unavailable", kind)
	}
	return engine.ContextResource{ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name, Subtype: item.Subtype, AgentRef: item.AgentRef, Config: item.Config}, nil
}

func (s *Service) kubernetesRuntimeConnection(ctx context.Context, resourceID string) (kclient.ConnectionInput, error) {
	item, err := s.runtimeResource(ctx, resourceID, "Kubernetes")
	if err != nil {
		return kclient.ConnectionInput{}, err
	}
	return s.kubernetesConnection(ctx, item)
}

func selectorValue(selector map[string]any, key string) string {
	value, _ := selector[key].(string)
	return strings.TrimSpace(value)
}

func appendAddress(values []string, host string, port int) []string {
	if port <= 0 || port > 65535 {
		return values
	}
	return append(values, net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port)))
}

func uniqueAddresses(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
