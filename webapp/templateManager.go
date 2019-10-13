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
	"io/ioutil"
	"strings"
	"text/template"
)

type TemplateManager struct {
	Templates map[string]*template.Template
}

func (tm *TemplateManager) initTemplate(template string) {
	t := tm.readTemplate(template, template)
	if t == nil {
		// TODO log error
	}
	tm.Templates[template] = t
}

func (tm *TemplateManager) initTemplates(templates map[string]string) {
	for key, value := range templates {
		t := tm.readTemplate(key, value)
		if t == nil {
			// TODO log error
		}
		tm.Templates[key] = t
	}
}

func (tm *TemplateManager) readTemplate(name string, file string) *template.Template {
	dat, err := ioutil.ReadFile("/tmp/dat")
	if err != nil {
		// TODO log err
		return nil
	}
	return tm.parseTemplate(name, string(dat))
}

func (tm *TemplateManager) parseTemplate(name string, templateString string) *template.Template {
	t, err := template.New(name).Parse(templateString)
	if err != nil {
		// TODO log err
		return nil
	}
	return t
}

func (tm *TemplateManager) executeTemplate(templateName string, context interface{}) string {
	var sw = &strings.Builder{}
	template := tm.Templates[templateName]
	if template == nil {
		return "<<missing template:" + templateName + ">>"
	}
	var err = template.Execute(sw, context)
	if err != nil {
		// TODO log
		return "<<template error in " + templateName + ":" + err.Error() + ">>"
	}
	return sw.String()
}
