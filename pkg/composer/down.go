/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package composer

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/containerd/nerdctl/pkg/strutil"

	"github.com/sirupsen/logrus"
)

type DownOptions struct {
	RemoveVolumes bool
}

func (c *Composer) Down(ctx context.Context, downOptions DownOptions) error {
	serviceNames, err := c.ServiceNames()
	if err != nil {
		return err
	}
	// reverse dependency order
	for _, svc := range strutil.ReverseStrSlice(serviceNames) {
		containers, err := c.Containers(ctx, svc)
		if err != nil {
			return err
		}
		if err := c.downContainers(ctx, containers, downOptions.RemoveVolumes); err != nil {
			return err
		}
	}

	for shortName := range c.project.Networks {
		if err := c.downNetwork(ctx, shortName); err != nil {
			return err
		}
	}

	if downOptions.RemoveVolumes {
		for shortName := range c.project.Volumes {
			if err := c.downVolume(ctx, shortName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Composer) downNetwork(ctx context.Context, shortName string) error {
	net, ok := c.project.Networks[shortName]
	if !ok {
		return fmt.Errorf("invalid network name %q", shortName)
	}
	if net.External.External {
		// NOP
		return nil
	}
	// shortName is like "default", fullName is like "compose-wordpress_default"
	fullName := net.Name
	netExists, err := c.NetworkExists(fullName)
	if err != nil {
		return err
	} else if netExists {
		logrus.Infof("Removing network %s", fullName)
		if err := c.runNerdctlCmd(ctx, "network", "rm", fullName); err != nil {
			logrus.Warn(err)
		}
	}
	return nil
}

func (c *Composer) downVolume(ctx context.Context, shortName string) error {
	vol, ok := c.project.Volumes[shortName]
	if !ok {
		return fmt.Errorf("invalid volume name %q", shortName)
	}
	if vol.External.External {
		// NOP
		return nil
	}
	// shortName is like "db_data", fullName is like "compose-wordpress_db_data"
	fullName := vol.Name
	volExists, err := c.VolumeExists(fullName)
	if err != nil {
		return err
	} else if volExists {
		logrus.Infof("Removing volume %s", fullName)
		if err := c.runNerdctlCmd(ctx, "volume", "rm", "-f", fullName); err != nil {
			logrus.Warn(err)
		}
	}
	return nil
}

func (c *Composer) downContainers(ctx context.Context, containers []containerd.Container, removeAnonVolumes bool) error {
	for _, container := range containers {
		info, _ := container.Info(ctx, containerd.WithoutRefreshedMetadata)
		logrus.Infof("Removing container %s", info.Labels[labels.Name])
		args := []string{"rm", "-f"}
		if removeAnonVolumes {
			args = append(args, "-v")
		}
		args = append(args, container.ID())
		if err := c.runNerdctlCmd(ctx, args...); err != nil {
			logrus.Warn(err)
		}
	}
	return nil
}
