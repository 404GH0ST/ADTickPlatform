package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"

	"adplatform/internal/services/apigateway"
)

func (e *dockerCLIExecutor) diskQuotaBytes() string {
	return strconv.FormatInt(int64(e.diskLimitMiB)<<20, 10)
}

func (e *dockerCLIExecutor) ensureStateVolume(ctx context.Context, volume string) error {
	if e.diskLimitMiB < 0 || int64(e.diskLimitMiB) > math.MaxInt64>>20 ||
		(e.requireDiskQuota && e.diskLimitMiB == 0) {
		return fmt.Errorf("CONTROLLER_SERVICE_DISK_LIMIT_MIB must specify a positive storage quota in MiB")
	}
	if e.diskLimitMiB == 0 {
		return nil
	}
	if volume == "" || e.stateMountPath == "" {
		return fmt.Errorf("quota-managed service requires a named state volume and mount path")
	}
	quota := e.diskQuotaBytes()
	if _, err := e.execDocker(ctx, "volume", "create", "--driver", "local", "--opt", "size="+quota, "--", volume); err != nil {
		return fmt.Errorf("provision quota-managed state volume: %w", err)
	}
	output, err := e.execDocker(ctx, "volume", "inspect", "--format", "{{.Driver}} {{json .Options}}", volume)
	if err != nil {
		return err
	}
	driver, optionsJSON, _ := strings.Cut(strings.TrimSpace(string(output)), " ")
	var options map[string]string
	if driver != "local" || json.Unmarshal([]byte(optionsJSON), &options) != nil || len(options) != 1 || options["size"] != quota {
		return fmt.Errorf("state volume %s needs offline migration to a local volume with quota %s bytes", volume, quota)
	}
	return nil
}

func (e *dockerCLIExecutor) verifyImageVolumes(ctx context.Context, image string) error {
	if e.diskLimitMiB <= 0 {
		return nil
	}
	output, err := e.execDocker(ctx, "image", "inspect", "--format", "{{json .Config.Volumes}}", image)
	if err != nil {
		return err
	}
	var volumes map[string]json.RawMessage
	if err := json.Unmarshal(output, &volumes); err != nil {
		return fmt.Errorf("inspect challenge image volume declarations: %w", err)
	}
	for mount := range volumes {
		if path.Clean(mount) != path.Clean(e.stateMountPath) {
			return fmt.Errorf("image volume %q must use the quota-managed state mount %q", mount, e.stateMountPath)
		}
	}
	return nil
}

func (e *dockerCLIExecutor) verifyContainerDiskLimits(ctx context.Context, name string) error {
	if e.diskLimitMiB <= 0 {
		return nil
	}
	output, err := e.execDocker(ctx, "inspect", "--format", `{{index .HostConfig.StorageOpt "size"}} {{.HostConfig.LogConfig.Type}} {{index .HostConfig.LogConfig.Config "max-size"}} {{index .HostConfig.LogConfig.Config "max-file"}}`, name)
	if err != nil {
		return err
	}
	if strings.Join(strings.Fields(string(output)), " ") != e.diskQuotaBytes()+" local 10m 3" {
		return fmt.Errorf("container %s needs offline recreation with writable-layer and log limits", name)
	}
	return nil
}

func (e *dockerCLIExecutor) verifyContainerDNS(ctx context.Context, task apigateway.ControllerRuntimeTask) error {
	if !task.DisableExternalDNS {
		return nil
	}
	output, err := e.execDocker(ctx, "inspect", "--format", `{{json .HostConfig.Dns}}`, task.ContainerName)
	if err != nil {
		return err
	}
	var servers []string
	if json.Unmarshal(output, &servers) != nil || len(servers) != 1 || servers[0] != "127.0.0.1" {
		return fmt.Errorf("container %s needs offline recreation with external DNS blocked", task.ContainerName)
	}
	return nil
}
