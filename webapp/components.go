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
	"github.com/sirupsen/logrus"
	"html"
	"strings"
)

type UIComponent interface {
	render() string
}

type UIContainer interface {
	UIComponent
	children() []interface{}
}

type Component struct {
	Styles map[string]string
	Class  string
}
type Container struct {
	Component
	Children []interface{}
}

func renderChildren(children []interface{}) string {
	b := strings.Builder{}
	for _, v := range children {
		switch v.(type) {
		case UIComponent:
			b.WriteString(v.(UIComponent).render())
		default:
			logrus.Warn("Ignoring non UIComponent type in child list.")
		}
	}
	return b.String()
}
func renderOpenTagWithClassAndStyle(tag string, class string, styles map[string]string) string {
	b := strings.Builder{}
	b.WriteString("<")
	b.WriteString(tag)
	if class != "" {
		b.WriteString(" class=\"" + class + "\"")
	}
	if len(styles) > 0 {
		b.WriteString(" style=\"")
		for k, v := range styles {
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
			b.WriteString(",")
		}
		b.WriteString("\"")
	}
	b.WriteString(">")
	return b.String()
}

type Body struct {
	Container
}

func (this Body) render() string {
	b := strings.Builder{}
	renderOpenTagWithClassAndStyle("body", this.Class, this.Styles)
	b.WriteString(renderChildren(this.Children))
	b.WriteString("</body>")
	return b.String()
}

type Div struct {
	Container
}

func (this Div) render() string {
	b := strings.Builder{}
	renderOpenTagWithClassAndStyle("div", this.Class, this.Styles)
	b.WriteString(renderChildren(this.Children))
	b.WriteString("</div>")
	return b.String()
}

type Paragraph struct {
	Component
	Text string
}

func (this Paragraph) render() string {
	return renderOpenTagWithClassAndStyle("p", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</p>"
}

type H1 struct {
	Component
	Text string
}

func (this H1) render() string {
	return renderOpenTagWithClassAndStyle("h1", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</h1>"
}

type H2 struct {
	Component
	Text string
}

func (this H2) render() string {
	return renderOpenTagWithClassAndStyle("h2", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</h2>"
}

type H3 struct {
	Component
	Text string
}

func (this H3) render() string {
	return renderOpenTagWithClassAndStyle("h3", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</h3>"
}

type H4 struct {
	Component
	Text string
}

func (this H4) render() string {
	return renderOpenTagWithClassAndStyle("h4p", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</h4>"
}

type Head struct {
	Meta  []string
	Links []string
	Title string
}

type H5 struct {
	Component
	Text string
}

func (this H5) render() string {
	return renderOpenTagWithClassAndStyle("h5", this.Class, this.Styles) +
		html.EscapeString(this.Text) + "</h5>"
}

type Link struct {
	Component
	Text   string
	Link   string
	Target string
}

func (this Link) render() string {
	return "<a href=\"" + this.Link + "\">" + html.EscapeString(this.Text) + "</a>"
}

type Break struct {
	Component
}

func (this Break) render() string {
	return "<br/>"
}
