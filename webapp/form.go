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

package webapp

import (
	"golang.org/x/text/message"
	"net/http"
)

type Form struct {
	Title       string
	TextInputs  map[string]*TextInput
	Options     map[string]*Options
	FormActions map[string]*FormAction
	Messages    *message.Printer
}

type FormAction struct {
	Name   string
	Value  string
	Method string
}

type TextInput struct {
	Name        string
	PlaceHolder string
	Value       string
}

func (ti TextInput) read(req *http.Request) {
	value := req.FormValue(ti.Name)
	if value != "" {
		ti.Value = value
	}
}

type Option struct {
	Name     string
	Value    string
	Selected bool
}

type Options struct {
	Entries []Option
}

func (op Options) Read(req *http.Request) {
	for _, option := range op.Entries {
		values := req.MultipartForm.Value[option.Value]
		if values != nil {
			option.Selected = true
		} else {
			option.Selected = false
		}
	}
}
