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
	"github.com/sirupsen/logrus"
	"github.com/winkube/util"
	"golang.org/x/text/language"
	"net/http"
	"net/url"
	"strings"
)

type WebApplication struct {
	Name            string
	templateManager *util.TemplateManager
	Pages           map[string]*Page
	Actions         map[string]*func(req *RequestContext, writer http.ResponseWriter) *ActionResponse
	sessionStore    *sessions.CookieStore
	Translations    *Translations
	rootContext     string
}

func CreateWebApp(name string, rootContext string, defaulLanguage language.Tag) *WebApplication {
	app := WebApplication{
		Name:            name,
		templateManager: util.CreateTemplateManager(),
		Pages:           make(map[string]*Page),
		Actions:         make(map[string]*func(req *RequestContext, writer http.ResponseWriter) *ActionResponse),
		rootContext:     rootContext,
		sessionStore:    sessions.NewCookieStore([]byte("WinKubeIsSoCool")),
		Translations:    CreateTranslations(defaulLanguage),
	}
	app.AddPage(&Page{
		application: app,
		Template:    "templates/_redirect.html",
		Name:        "_redirect",
	})
	app.sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   1000 * 50 * 30, // 30 minutes
		HttpOnly: true,
	}
	return &app
}

func (app *WebApplication) LoadTranslations(lang language.Tag) *WebApplication {
	app.Translations.load(lang)
	return app
}

func (app *WebApplication) SetAction(name string, action func(req *RequestContext, writer http.ResponseWriter) *ActionResponse) *WebApplication {
	app.Actions[name] = &action
	return app
}

func (app *WebApplication) HandleRequest(writer http.ResponseWriter, req *http.Request) {
	// TODO Get language
	langs := app.GetLanguages(req)
	var language language.Tag = langs[0]
	// get action...
	// Get a session. Get() always returns a session, even if empty.
	session, err := app.sessionStore.Get(req, "app-"+app.Name)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	// Save session at end
	defer session.Save(req, writer)
	var renderModel *RenderModel = &RenderModel{
		Messages: app.Translations.Properties(language),
	}
	renderModel.Context = &RequestContext{
		Application: app,
		Request:     req,
		Session:     session,
		Language:    language,
	}

	var action *func(req *RequestContext, writer http.ResponseWriter) *ActionResponse = app.findAction(req)

	if action != nil {
		actionResponse := (*action)(renderModel.Context, writer)
		if actionResponse.NextPage != "" {
			nextPage, found := app.Pages[actionResponse.NextPage]
			if !found {
				panic("Invalid page: " + actionResponse.NextPage)
			}
			renderModel.Page = nextPage
		}
		if actionResponse.Model != nil {
			renderModel.Data = actionResponse.Model
		}
		if actionResponse.complete {
			return
		}
	}
	// no action, try to find page...
	if renderModel.Page == nil {
		renderModel.Page = app.findPage(req)
	}
	if renderModel.Page != nil {
		renderedPage := renderModel.Page.render(renderModel)
		buf := bytes.NewBufferString(renderedPage)
		writer.Write(buf.Bytes())
	}
}

func (app *WebApplication) GetLanguages(req *http.Request) []language.Tag {
	langHeader := req.Header.Get("Accept-Language")
	if langHeader == "" {
		return []language.Tag{app.Translations.DefaultLanguage}
	}
	var result []language.Tag
	values := strings.Split(langHeader, ",")
	for _, v := range values {
		langs := strings.Split(v, ";")
		tag, err := language.Parse(langs[0])
		if err != nil {
			logrus.Warn("Inpuarseable language tag in Accept-Language header or %s: %s", req, err)
		} else {
			result = append(result, tag)
		}
	}
	return result
}

func (app *WebApplication) InitTemplates(templates map[string]string) *WebApplication {
	app.templateManager.InitTemplates(templates)
	return app
}

func (app *WebApplication) AddPage(page *Page) *WebApplication {
	// Check for existing template
	if app.templateManager.Templates[page.Template] == nil {
		app.templateManager.InitTemplate(page.Template)
	}
	page.application = *app
	app.Pages[page.Name] = page
	return app
}

func (app *WebApplication) findAction(req *http.Request) *func(req *RequestContext, writer http.ResponseWriter) *ActionResponse {
	uri, _ := url.ParseRequestURI(req.RequestURI)
	var action *func(req *RequestContext, writer http.ResponseWriter) *ActionResponse
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

func (app *WebApplication) ExecuteTemplate(template string, model *RenderModel) string {
	return app.templateManager.ExecuteTemplate(template, model)
}

type RequestContext struct {
	Application *WebApplication
	Request     *http.Request
	Attributes  map[string]interface{}
	Session     *sessions.Session
	Language    language.Tag
}

func (this RequestContext) GetParameter(key string) string {
	param := this.GetQueryParameter(key)
	if param == "" {
		param = this.GetFormParameter(key)
	}
	//if(param == ""){
	//	param = this.GetSessionAttribute(key).(string)
	//}
	//if(param == ""){
	//	param = this.getRequestAttribute(key).(string)
	//}
	return param
}

func (this RequestContext) GetParameterOrDefault(key string, defaultValue string) string {
	val := this.GetParameter(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func (this RequestContext) GetFormParameters(key string) []string {
	if this.Request.MultipartForm != nil {
		return this.Request.MultipartForm.Value[key]
	}
	return nil
}

func (this RequestContext) GetFormParameter(key string) string {
	params := this.GetFormParameters(key)
	if params != nil && len(params) > 0 {
		return params[0]
	}
	return ""
}

func (this RequestContext) GetQueryParameter(key string) string {
	return this.GetQueryParameterWithDefault(key, "")
}

func (this RequestContext) GetQueryParameterWithDefault(key string, defaultValue string) string {
	uri, err := url.ParseRequestURI(this.Request.RequestURI)
	if err != nil {
		panic("Cannot read request URI: " + this.Request.RequestURI + ", err: " + err.Error())
	}
	params, found := uri.Query()[key]
	if found {
		return params[0]
	}
	return defaultValue
}

func (this RequestContext) GetSessionAttribute(key string) interface{} {
	return this.GetSessionAttributeWithDefault(key, nil)
}

func (this RequestContext) GetSessionAttributeWithDefault(key string, defaultValue interface{}) interface{} {
	if this.Session != nil {
		v, found := this.Session.Values[key]
		if found {
			return v.(string)
		}
	}
	return defaultValue
}

func (this RequestContext) SetSessionAttribute(key string, value interface{}) interface{} {
	if this.Session != nil {
		v := this.Session.Values[key]
		this.Session.Values[key] = value
		return v
	}
	return nil
}

func (this RequestContext) getRequestAttribute(key string) interface{} {
	return this.getRequestAttributeWithDefault(key, nil)
}

func (this RequestContext) getRequestAttributeWithDefault(key string, defaultValue interface{}) interface{} {
	if this.Attributes != nil {
		v := this.Attributes[key]
		if v != nil {
			return v
		}
	}
	return defaultValue
}
func (this RequestContext) setRequestAttribute(key string, value interface{}) interface{} {
	if this.Attributes != nil {
		this.Attributes = make(map[string]interface{})
	}
	oldVal := this.Attributes[key]
	this.Attributes[key] = value
	return oldVal
}
