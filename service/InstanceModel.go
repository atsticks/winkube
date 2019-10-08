// Copyright 2019 Anatole Tresch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"github.com/google/uuid"
	"github.com/winkube/service/netutil"
	"os"
	"sync"
)

const UNDEFINED_ROLE string = "undefined"
const MASTER_ROLE string = "master"
const NODE_ROLE string = "node"

var (
	instance     InstanceModel
	instanceOnce sync.Once
)

type InstanceModel struct {
	id           string `json:"id"`
	InstanceRole string `json:"role"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
}

func (model InstanceModel) Id() string {
	return model.id
}

func GetInstanceModel() InstanceModel {
	instanceOnce.Do(func() {
		instance = InstanceModel{
			InstanceRole: UNDEFINED_ROLE,
			Name:         hostname(),
			Host:         netutil.GetInternalIP(),
			Port:         9999,
			id:           uuid.New().String(),
		}
	})
	return instance
}

func hostname() string {
	var hn string
	hn, _ = os.Hostname()
	return hn
}
