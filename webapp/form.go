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
