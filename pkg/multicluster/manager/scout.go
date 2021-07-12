/*
Copyright 2021 KubeCube Authors

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

package manager

import (
	"context"
	"time"

	"github.com/kubecube-io/kubecube/pkg/clog"

	"github.com/kubecube-io/kubecube/pkg/utils/exit"
)

// ScoutFor starts watch for warden intelligence
func (m *MultiClustersMgr) ScoutFor(ctx context.Context, cluster string) error {
	c, err := m.Get(cluster)
	if err != nil {
		return err
	}

	c.Scout.Once.Do(func() {
		clog.Info("start scout for cluster %v", c.Scout.Cluster)

		ctx = exit.SetupCtxWithStop(ctx, c.Scout.StopCh)

		time.AfterFunc(time.Duration(c.Scout.InitialDelaySeconds)*time.Second, func() {
			go c.Scout.Collect(ctx)
		})
	})

	return nil
}
