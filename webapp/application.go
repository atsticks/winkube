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
	"bytes"
	"github.com/gorilla/sessions"
	"net/http"
	"net/url"
	"strings"
)

type WebApplication struct {
	Name            string
	templateManager *TemplateManager
	Pages           map[string]*Page
	Actions         map[string]*Action
	sessionStore    *sessions.CookieStore
	rootContext     string
}

func Create(name string, rootContext string) *WebApplication {
	app := WebApplication{
		Name:            name,
		templateManager: NewTemplateManager(),
		Pages:           make(map[string]*Page),
		Actions:         make(map[string]*Action),
		rootContext:     rootContext,
		sessionStore:    sessions.NewCookieStore([]byte("WinKubeIsSoCool")),
	}
	app.sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   1000 * 50 * 30, // 30 minutes
		HttpOnly: true,
	}
	return &app
}

func (app *WebApplication) SetAction(name string, action Action) *WebApplication {
	app.Actions[name] = &action
	return app
}

func (app *WebApplication) HandleRequest(writer http.ResponseWriter, req *http.Request) {
	// get action...
	// Get a session. Get() always returns a session, even if empty.
	session, err := app.sessionStore.Get(req, "app-"+app.Name)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	var action *Action = app.findAction(req)
	var actionResponse *ActionResponse
	if action != nil {
		context := &RequestContext{
			Application: app,
			Request:     req,
		}
		actionResponse := (*action).DoAction(context, writer)
		if actionResponse.NextPage != "" {
			nextPage, found := app.Pages[actionResponse.NextPage]
			if !found {
				panic("Invalid page: " + actionResponse.NextPage)
			}
			renderedPage := nextPage.render(actionResponse.Model)
			buf := bytes.NewBufferString(renderedPage)
			writer.Write(buf.Bytes())
			return
		}
	}
	// get page...
	var page *Page = app.findPage(req)
	if page != nil {
		var pageModel *interface{}
		if actionResponse != nil {
			pageModel = actionResponse.Model
		}
		renderedPage := page.render(pageModel)
		buf := bytes.NewBufferString(renderedPage)
		writer.Write(buf.Bytes())
	}
	// Save it before we write to the response/return from the handler.
	err = session.Save(req, writer)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (app *WebApplication) InitTemplates(templates map[string]string) *WebApplication {
	app.templateManager.initTemplates(templates)
	return app
}

func (app *WebApplication) AddPage(page *Page) *WebApplication {
	// Check for existing template
	if app.templateManager.Templates[page.Template] == nil {
		app.templateManager.initTemplate(page.Template)
	}
	page.application = *app
	app.Pages[page.Name] = page
	return app
}

func (app *WebApplication) findAction(req *http.Request) *Action {
	uri, _ := url.ParseRequestURI(req.RequestURI)
	var action *Action
	actionName, found := uri.Query()["action"]
	if found {
		action = app.Actions[actionName[0]]
	}
	path := uri.Path
	if strings.Index(path, app.rootContext) == 0 {
		path = strings.TrimPrefix(path, app.rootContext)
	}
	if !(strings.Index(path, "/") == 0) {
		path = "/" + path
	}
	action = app.Actions[path]
	if action == nil {
		// check subsplits
		parts := strings.SplitN(path, "/", 1)
		for len(parts) > 1 {
			action = app.Actions[parts[1]]
			if action != nil {
				return action
			}
			parts = strings.SplitN(parts[1], "/", 1)
		}
	}
	return action
}

func (app *WebApplication) findPage(req *http.Request) *Page {
	uri, _ := url.ParseRequestURI(req.RequestURI)
	var page *Page
	pageName, found := uri.Query()["page"]
	if found {
		page = app.Pages[pageName[0]]
	}
	path := uri.Path
	if strings.Index(path, app.rootContext) == 0 {
		path = strings.TrimPrefix(path, app.rootContext)
	}
	if !(strings.Index(path, "/") == 0) {
		path = "/" + path
	}
	page = app.Pages[path]
	if page == nil {
		// check subsplits
		parts := strings.SplitN(path, "/", 1)
		for len(parts) > 1 {
			page = app.Pages[parts[1]]
			if page != nil {
				return page
			}
			parts = strings.SplitN(parts[1], "/", 1)
		}
	}
	return page
}
