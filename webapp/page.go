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
	"net/http"
)

type Page struct {
	application WebApplication
	Name        string
	Template    string
	Title       string
}

type PageModel struct {
	Page *Page
	Data interface{}
}

func (page *Page) render(context *interface{}) string {
	return page.application.templateManager.executeTemplate(page.Template, PageModel{
		Page: page,
		Data: context,
	})
}

type RequestContext struct {
	Application *WebApplication
	Request     *http.Request
}

type ActionResponse struct {
	NextPage *Page
	Model    *interface{}
	complete bool
}

type Action interface {
	doAction(req *RequestContext, writer http.ResponseWriter) *ActionResponse
}
