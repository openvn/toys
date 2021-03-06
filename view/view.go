// Copyright 2012 The Toys Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package view

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/openvn/toys/locale"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
)

type ViewSet struct {
	diver *template.Template
	page  map[string]*template.Template
}

func NewViewSet() *ViewSet {
	v := &ViewSet{}
	v.page = make(map[string]*template.Template)
	return v
}

type View struct {
	root     string
	set      map[string]*ViewSet
	current  string
	funcsMap template.FuncMap
	resource string
}

func NewView(root string) *View {
	v := &View{}
	v.root = root
	v.set = make(map[string]*ViewSet)
	v.funcsMap = template.FuncMap{}
	v.funcsMap["resource"] = func(uri string) string {
		return v.resource + uri
	}
	v.funcsMap["equal"] = func(a, b interface{}) bool {
		return a == b
	}
	v.funcsMap["plus"] = func(a, b int) int {
		return a + b
	}
	v.funcsMap["indent"] = func(s string, n int) string {
		var buff bytes.Buffer
		for i := 0; i < n; i++ {
			buff.WriteString(s)
		}
		return buff.String()
	}
	return v
}

func (v *View) HandleResource(prefix, folder string) {
	v.resource = prefix
	http.Handle(prefix, http.StripPrefix(prefix,
		http.FileServer(http.Dir(folder))))
}

func (v *View) AddFunc(name string, f interface{}) error {
	if r := reflect.TypeOf(f); r.Kind() == reflect.Func {
		if r.NumOut() > 2 {
			return fmt.Errorf("view: %s", "function must have no more than 2 output parameter")
		}
		if r.NumOut() == 2 {
			o := r.Out(1)
			_, ok := o.MethodByName("Error")
			if !ok {
				return fmt.Errorf("view: %s", "function must have the last output parameter implements error")
			}
		}
		v.funcsMap[name] = f
		return nil
	}
	return fmt.Errorf("view: %s", "AddFunc require a valid function")
}

func (v *View) SetDefault(set string) error {
	_, ok := v.set[set]
	if !ok {
		err := v.Parse(set)
		if err != nil {
			return err
		}
	}
	v.current = set
	return nil
}

func (v *View) Parse(set string) error {
	setFolder := filepath.Join(v.root, set)

	tmpl := template.Must(template.New("layout.tmpl").Funcs(v.funcsMap).
		ParseGlob(filepath.Join(setFolder, "shared", "*.tmpl")))
	vs := NewViewSet()
	vs.diver = tmpl
	//parse page
	setroot, err := os.Open(setFolder)
	defer setroot.Close()

	if err != nil {
		return err
	}

	files, err := setroot.Readdir(-1)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.IsDir() {
			p, err := tmpl.Clone()
			if err != nil {
				continue
			}
			//read file
			b, err := ioutil.ReadFile(filepath.Join(setFolder, file.Name()))
			if err != nil {
				continue
			}
			_, err = p.Parse(string(b))
			if err == nil {
				vs.page[file.Name()] = p
			} else {
				return err
			}
		}
	}

	v.set[set] = vs
	v.current = set
	return nil
}

func (v *View) Load(w io.Writer, pageName string, data interface{}) error {
	p, ok := v.set[v.current].page[pageName]
	if ok {
		return p.ExecuteTemplate(w, "layout.tmpl", data)
	}
	fmt.Fprintf(w, "%#v", data)
	return errors.New("view: cannot load template")
}

func (v *View) SetLang(l *locale.Lang) {
	v.funcsMap["lang"] = func(file, key string) string {
		return l.Load(file, key)
	}
	v.funcsMap["langset"] = func(set, file, key string) string {
		return l.LoadSet(set, file, key)
	}
}
