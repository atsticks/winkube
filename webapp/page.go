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

package webapp

import (
	"github.com/winkube/service/runtime"
	"net/http"
)

type Page struct {
	application WebApplication
	Template    string
	Name        string
}

type RenderModel struct {
	Page     *Page
	Context  *RequestContext
	Messages map[string]string
	Data     map[string]interface{}
}

func (page *Page) render(model *RenderModel) string {
	runtime.Container().Logger.Debug("Rendering page: " + page.Name)
	model.Page = page
	return page.application.templateManager.executeTemplate(page.Template, model)
}

type ActionResponse struct {
	NextPage string
	Model    map[string]interface{}
	complete bool
}

type Action interface {
	DoAction(req *RequestContext, writer http.ResponseWriter) *ActionResponse
}
