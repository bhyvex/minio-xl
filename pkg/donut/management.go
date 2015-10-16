/*
 * Minio Cloud Storage, (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package donut

import (
	"github.com/minio/minio-xl/pkg/donut/disk"
	"github.com/minio/minio-xl/pkg/probe"
)

// Info - return info about donut configuration
func (donut API) Info() (nodeDiskMap map[string][]string, err *probe.Error) {
	nodeDiskMap = make(map[string][]string)
	for nodeName, n := range donut.nodes {
		disks, err := n.ListDisks()
		if err != nil {
			return nil, err.Trace()
		}
		diskList := make([]string, len(disks))
		for diskOrder, disk := range disks {
			diskList[diskOrder] = disk.GetPath()
		}
		nodeDiskMap[nodeName] = diskList
	}
	return nodeDiskMap, nil
}

// AttachNode - attach node
func (donut API) AttachNode(hostname string, disks []string) *probe.Error {
	if hostname == "" || len(disks) == 0 {
		return probe.NewError(InvalidArgument{})
	}
	n, err := newNode(hostname)
	if err != nil {
		return err.Trace()
	}
	donut.nodes[hostname] = n
	for i, d := range disks {
		newDisk, err := disk.New(d)
		if err != nil {
			continue
		}
		if err := newDisk.MakeDir(donut.config.DonutName); err != nil {
			return err.Trace()
		}
		if err := n.AttachDisk(newDisk, i); err != nil {
			return err.Trace()
		}
	}
	return nil
}

// DetachNode - detach node
func (donut API) DetachNode(hostname string) *probe.Error {
	delete(donut.nodes, hostname)
	return nil
}

// Rebalance - rebalance an existing donut with new disks and nodes
func (donut API) Rebalance() *probe.Error {
	return probe.NewError(APINotImplemented{API: "management.Rebalance"})
}

// Heal - heal your donuts
func (donut API) Heal() *probe.Error {
	// TODO handle data heal
	return donut.healBuckets()
}
