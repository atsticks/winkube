package webapp

import "net/http"

type Form struct {
	Title       string
	TextInputs  map[string]*TextInput
	Options     map[string]*Options
	FormActions map[string]*FormAction
}

type FormInput struct {
	Name string
	Help string
}

type FormAction struct {
	FormInput
	value  string
	method string
}

type TextInput struct {
	FormInput
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
	FormInput
	Value    string
	Selected bool
}

type Options struct {
	FormInput
	Selected []string
	Entries  []Option
}

func (op Options) read(req *http.Request) {
	op.Selected = []string{}
	for _, option := range op.Entries {
		values := req.MultipartForm.Value[option.Value]
		if values != nil {
			for _, v := range values {
				op.Selected = append(op.Selected, v)
			}
		}
	}
}
