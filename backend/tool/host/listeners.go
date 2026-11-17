package host

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Listeners reads TCP and UDP listening sockets owned by one Linux process.
// It uses procfs so the same bounded read path works for local and SSH hosts.
func Listeners(ctx context.Context, input ListenersInput) (ListenersOutput, error) {
	if input.PID <= 0 {
		return ListenersOutput{}, fmt.Errorf("%w: pid must be positive", ErrInvalidArgument)
	}
	src, target, err := openSource(contextOrBackground(ctx), input.ConnectionInput)
	if err != nil {
		return ListenersOutput{}, err
	}
	defer src.Close()
	output := ListenersOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target, PID: input.PID, Listeners: []Listener{}}
	path := filepath.Join("/proc", strconv.Itoa(input.PID))
	inodes, err := processSocketInodes(ctx, src, filepath.Join(path, "fd"))
	if err != nil {
		return output, nil
	}
	for _, table := range []struct {
		path, protocol, state string
	}{
		{filepath.Join(path, "net", "tcp"), "tcp", "0A"},
		{filepath.Join(path, "net", "tcp6"), "tcp", "0A"},
		{filepath.Join(path, "net", "udp"), "udp", "07"},
		{filepath.Join(path, "net", "udp6"), "udp", "07"},
	} {
		raw, readErr := readLimited(ctx, src, table.path)
		if readErr != nil {
			output.Partial = true
			output.Unavailable = append(output.Unavailable, table.protocol)
			continue
		}
		output.Listeners = append(output.Listeners, parseListenerTable(string(raw), inodes, table.protocol, table.state)...)
	}
	sort.Slice(output.Listeners, func(i, j int) bool {
		if output.Listeners[i].Port == output.Listeners[j].Port {
			return output.Listeners[i].Protocol < output.Listeners[j].Protocol
		}
		return output.Listeners[i].Port < output.Listeners[j].Port
	})
	return output, nil
}

func processSocketInodes(ctx context.Context, src source, path string) (map[string]struct{}, error) {
	entries, err := src.ReadDir(ctx, path)
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		value, linkErr := src.ReadLink(ctx, entry)
		if linkErr != nil || !strings.HasPrefix(value, "socket:[") || !strings.HasSuffix(value, "]") {
			continue
		}
		result[strings.TrimSuffix(strings.TrimPrefix(value, "socket:["), "]")] = struct{}{}
	}
	return result, nil
}

func parseListenerTable(raw string, inodes map[string]struct{}, protocol, state string) []Listener {
	seen := map[string]struct{}{}
	result := []Listener{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 11 || fields[3] != state {
			continue
		}
		if _, found := inodes[fields[9]]; !found {
			continue
		}
		_, rawPort, found := strings.Cut(fields[1], ":")
		if !found {
			continue
		}
		port, err := strconv.ParseInt(rawPort, 16, 32)
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		key := protocol + ":" + strconv.Itoa(int(port))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, Listener{Port: int(port), Protocol: protocol})
	}
	return result
}
