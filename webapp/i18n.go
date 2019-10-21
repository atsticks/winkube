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
	properties "github.com/magiconair/properties"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

type Translations struct {
	properties      map[language.Tag]*properties.Properties
	DefaultLanguage language.Tag
}

func CreateTranslations(defaultLanguage language.Tag, languages ...language.Tag) *Translations {
	t := Translations{
		properties: make(map[language.Tag]*properties.Properties),
	}
	t.DefaultLanguage = defaultLanguage
	t.load(defaultLanguage)
	// Only write log, no panic for non default languages!
	properties.ErrorHandler = func(err error) {
		logrus.Error("Error loading language translations: %s", err.Error())
	}
	for _, lang := range languages {
		t.load(lang)
	}
	return &t
}

func (this Translations) load(lang language.Tag) {
	_, exists := this.properties[lang]
	if !exists {
		p, err := properties.LoadFile("i18n/translations-"+lang.String()+".properties", properties.UTF8)
		if err == nil {
			this.properties[lang] = p
			return
		}
		logrus.Error("Error loading language translations for %s: %s", lang.String(), err.Error())
	}
}

func (this Translations) Properties(lang language.Tag) map[string]string {
	p := this.properties[lang]
	if p == nil {
		if lang != this.DefaultLanguage {
			return this.Properties(this.DefaultLanguage)
		}
		panic("Unsupported language: " + lang.String())
	}
	return p.Map()
}

func (this Translations) PropertiesWithDefaults(lang language.Tag, messageKeysAndDefaults ...string) map[string]string {
	var curKey string
	// build app defaults map
	defaultValues := make(map[string]string)
	for _, v := range messageKeysAndDefaults {
		if curKey == "" {
			curKey = v
		} else {
			defaultValues[curKey] = v
			curKey = ""
		}
	}
	// Get all properties and override any missing values with defaults...
	result := this.Properties(lang)
	for k, v := range defaultValues {
		if result[k] == "" {
			result[k] = v
		}
	}
	return result
}

func (this Translations) PropertiesWithOverrides(lang language.Tag, messageKeysAndOverrides ...string) map[string]string {
	var curKey string
	// build app overrides map
	overrideValues := make(map[string]string)
	for _, v := range messageKeysAndOverrides {
		if curKey == "" {
			curKey = v
		} else {
			overrideValues[curKey] = v
			curKey = ""
		}
	}
	// Get all properties and override any missing values with defaults...
	result := this.Properties(lang)
	for k, v := range overrideValues {
		result[k] = v
	}
	return result
}

func (this Translations) LookupMessage(lang language.Tag, key string) string {
	return this.LookupMessageWithDefault(lang, key, "--message:"+key+"--")
}

func (this Translations) LookupMessageWithDefault(lang language.Tag, key string, defaultValue string) string {
	val := this.Properties(lang)[key]
	if val == "" {
		return defaultValue
	}
	return val
}
