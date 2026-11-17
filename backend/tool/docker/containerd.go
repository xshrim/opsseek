package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"opsseek/backend/mcpserver/docker/client"
)

func withContainerd(ctx context.Context, input client.ConnectionInput, fn func(*containerd.Client, context.Context) error) error {
	cli, err := client.OpenContainerd(input)
	if err != nil {
		return err
	}
	defer cli.Close()
	ns := strings.TrimSpace(input.Namespace)
	if ns == "" {
		ns = "k8s.io"
	}
	ctx = namespaces.WithNamespace(ctx, ns)
	if timeout := client.TimeoutSeconds(input.TimeoutSeconds); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return fn(cli, ctx)
}

func containerdInfo(ctx context.Context, input client.ConnectionInput) (DockerInfoOutput, error) {
	var out DockerInfoOutput
	err := withContainerd(ctx, input, func(cli *containerd.Client, ctx context.Context) error {
		version, err := cli.Version(ctx)
		if err != nil {
			return err
		}
		containers, err := cli.Containers(ctx)
		if err != nil {
			return err
		}
		images, err := cli.ListImages(ctx)
		if err != nil {
			return err
		}
		out.Info = DockerInfoDTO{Name: "containerd", ServerVersion: version.Version, Images: len(images), Containers: len(containers), Warnings: []string{"containerd gRPC API: Docker-only logging/network fields are unavailable"}}
		for _, item := range containers {
			task, err := item.Task(ctx, nil)
			if err == nil {
				status, statusErr := task.Status(ctx)
				if statusErr == nil && status.Status == containerd.Running {
					out.Info.ContainersRunning++
				} else {
					out.Info.ContainersStopped++
				}
			} else {
				out.Info.ContainersStopped++
			}
		}
		return nil
	})
	return out, err
}

func containerdImages(ctx context.Context, input ListImagesInput) (ImagesOutput, error) {
	if strings.TrimSpace(input.Filters) != "" {
		return ImagesOutput{}, fmt.Errorf("containerd image filters are not supported; omit filters")
	}
	out := ImagesOutput{Images: []ImageDTO{}}
	err := withContainerd(ctx, input.ConnectionInput, func(cli *containerd.Client, ctx context.Context) error {
		items, err := cli.ListImages(ctx)
		if err != nil {
			return err
		}
		for _, item := range items {
			if !input.All && strings.Contains(item.Name(), "@sha256:") {
				continue
			}
			size, _ := item.Size(ctx)
			out.Images = append(out.Images, ImageDTO{ID: item.Target().Digest.String(), RepoTags: []string{item.Name()}, Created: item.Metadata().CreatedAt.Unix(), SizeBytes: size, Labels: item.Labels()})
		}
		return nil
	})
	return out, err
}

func containerdContainers(ctx context.Context, input ListContainersInput) (ContainersOutput, error) {
	if input.Limit < 0 {
		return ContainersOutput{}, fmt.Errorf("limit must not be negative")
	}
	if strings.TrimSpace(input.Filters) != "" {
		return ContainersOutput{}, fmt.Errorf("containerd container filters are not supported; omit filters")
	}
	if input.Limit > maxListLimit {
		input.Limit = maxListLimit
	}
	out := ContainersOutput{Containers: []ContainerDTO{}}
	err := withContainerd(ctx, input.ConnectionInput, func(cli *containerd.Client, ctx context.Context) error {
		items, err := cli.Containers(ctx)
		if err != nil {
			return err
		}
		for _, item := range items {
			info, err := item.Info(ctx)
			if err != nil {
				return err
			}
			task, taskErr := item.Task(ctx, nil)
			state := "created"
			if taskErr == nil {
				status, statusErr := task.Status(ctx)
				if statusErr == nil {
					state = string(status.Status)
				}
			}
			if !input.All && state != "running" {
				continue
			}
			out.Containers = append(out.Containers, ContainerDTO{ID: item.ID(), Names: []string{item.ID()}, Image: info.Image, ImageID: info.Image, Created: info.CreatedAt.Unix(), Labels: info.Labels, State: state, Status: state})
			if input.Limit > 0 && len(out.Containers) >= input.Limit {
				break
			}
		}
		return nil
	})
	return out, err
}

func containerdInspect(ctx context.Context, input ContainerInspectInput) (ContainerInspectDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return ContainerInspectDTO{}, err
	}
	var out ContainerInspectDTO
	err = withContainerd(ctx, input.ConnectionInput, func(cli *containerd.Client, ctx context.Context) error {
		item, err := cli.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		info, err := item.Info(ctx)
		if err != nil {
			return err
		}
		spec, err := item.Spec(ctx)
		if err != nil {
			return err
		}
		state := &ContainerStateDTO{Status: "created"}
		task, taskErr := item.Task(ctx, nil)
		if taskErr == nil {
			status, statusErr := task.Status(ctx)
			if statusErr == nil {
				state.Status = string(status.Status)
				state.Running = status.Status == containerd.Running
				state.PID = int(task.Pid())
				state.ExitCode = int(status.ExitStatus)
				if !status.ExitTime.IsZero() {
					state.FinishedAt = status.ExitTime.Format(time.RFC3339Nano)
				}
			}
		}
		config := &ContainerConfigDTO{Image: info.Image, Labels: info.Labels}
		if spec.Process != nil {
			config.User = spec.Process.User.Username
			config.WorkingDir = spec.Process.Cwd
			config.Command = append([]string(nil), spec.Process.Args...)
			config.TTY = spec.Process.Terminal
			for _, e := range spec.Process.Env {
				if i := strings.IndexByte(e, '='); i > 0 {
					config.EnvNames = append(config.EnvNames, e[:i])
				}
			}
		}
		out = ContainerInspectDTO{ID: info.ID, Name: info.ID, Created: info.CreatedAt.Format(time.RFC3339Nano), Image: info.Image, State: state, Config: config}
		return nil
	})
	if err != nil {
		return ContainerInspectDTO{}, fmt.Errorf("inspect containerd container: %w", err)
	}
	return out, nil
}
