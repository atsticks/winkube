// Copyright 2019 Anatole Tresch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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
	"time"
)

type InstanceModel struct {
	id           string `json:"id"`
	InstanceRole string `json:"role"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
}

func (model InstanceModel) Id() string {
	if model.id == "" {
		model.id = uuid.New().String()
	}
	return model.id
}

func hostname() string {
	var hn string
	hn, _ = os.Hostname()
	return hn
}

type RegisteredInstance struct {
	InstanceModel
	timestamp int
}

func RegisteredInstance_fromService(s netutil.Service) *RegisteredInstance {
	return &RegisteredInstance{
		InstanceModel: InstanceModel{
			id:           s.Id,
			InstanceRole: s.Service,
			Host:         s.Host(),
			Port:         s.Port(),
		},
		timestamp: time.Now().Nanosecond() / 1000,
	}
}

type Master struct {
	RegisteredInstance
	Labels map[string]string `json:"labels"`
}

type Node struct {
	RegisteredInstance
	Labels map[string]string `json:"labels"`
}
