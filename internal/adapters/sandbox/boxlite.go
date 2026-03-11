package sandbox

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
)

type BoxLiteSandbox struct {
	Base Sandbox

	Command string
	Image   string
	RunArgs []string
	CPUs    string
	Memory  string
	Network string
}

func (s BoxLiteSandbox) Prepare(ctx context.Context, in PrepareInput) (acpclient.LaunchConfig, error) {
	return prepareContainerLaunch(ctx, s.Base, in, containerLaunchSpec{
		command: s.Command,
		image:   s.Image,
		runArgs: append([]string(nil), s.RunArgs...),
		cpus:    s.CPUs,
		memory:  s.Memory,
		network: s.Network,
	}, buildBoxLiteArgs)
}

func buildBoxLiteArgs(spec containerLaunchSpec, launch acpclient.LaunchConfig, mounts []containerVolume) []string {
	args := make([]string, 0, 16+len(spec.runArgs)+len(mounts)*2+len(launch.Env)*2+len(launch.Args))
	args = append(args, "run", "--rm", "-i")
	if spec.cpus != "" {
		args = append(args, "--cpus", spec.cpus)
	}
	if spec.memory != "" {
		args = append(args, "--memory", spec.memory)
	}
	if spec.network != "" {
		args = append(args, "--network", spec.network)
	}
	for _, mount := range mounts {
		args = append(args, "-v", mount.hostPath+":"+mount.containerPath)
	}
	if launch.WorkDir != "" {
		args = append(args, "-w", launch.WorkDir)
	}
	args = appendSortedEnvArgs(args, launch.Env, "-e")
	args = append(args, spec.runArgs...)
	args = append(args, spec.image, launch.Command)
	args = append(args, launch.Args...)
	return args
}
