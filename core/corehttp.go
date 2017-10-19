package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/covrom/gonec/names"
)

type VMHttpRequest struct {
	r    *http.Request
	data VMValuer
}

func (x *VMHttpRequest) vmval() {}

func (x *VMHttpRequest) Interface() interface{} {
	return x.r
}

func (x *VMHttpRequest) String() string {
	return fmt.Sprintf("Запрос %s %s %s", x.r.RemoteAddr, x.r.Method, x.r.RequestURI)
}

func (x *VMHttpRequest) GetHeader(name VMString) VMString {
	return VMString(x.r.Header.Get(string(name)))
}

func (x *VMHttpRequest) SetHeader(name, val VMString) {
	x.r.Header.Set(string(name), string(val))
}

func (x *VMHttpRequest) ReadBody() (VMString, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, x.r.Body)
	if err != nil {
		return VMString(""), err
	}
	return VMString(buf.String()), nil
}

func (x *VMHttpRequest) Path() VMString {
	return VMString(x.r.URL.Path)
}

func (x *VMHttpRequest) RemoteAddr() VMString {
	return VMString(x.r.RemoteAddr)
}

func (x *VMHttpRequest) Fragment() VMString {
	return VMString(x.r.URL.Fragment)
}

func (x *VMHttpRequest) GetParam(name VMString) VMString {
	return VMString(x.r.FormValue(string(name)))
}

func (x *VMHttpRequest) Method() VMString {
	return VMString(x.r.Method)
}

// RequestAsVMStringMap возвращает структуру в формате JSON
// {
//  "Адрес":"IP адрес корреспондента",
//  "Путь":"/root",
//  "Фрагмент":"после#",
//  "Параметры":{"Имя":Значение,...},
//  "ПараметрыФормы":{"Имя":Значение,...},
//  "Метод":Метод,
//  "Заголовки":{"Имя":Значение,...},
//  "Тело":"Строка JSON"
// }
func (x *VMHttpRequest) RequestAsVMStringMap() (VMStringMap, error) {

	rmap := make(VMStringMap)

	err := x.r.ParseMultipartForm(32 << 20)
	// if err != nil {
	// 	return rmap, err
	// }

	rmap["Тело"], err = x.ReadBody()
	if err != nil {
		return rmap, err
	}

	rmap["Адрес"] = x.RemoteAddr()
	rmap["Путь"] = x.Path()
	rmap["Фрагмент"] = x.Fragment()
	// rmap["Данные"] = x.data
	rmap["Метод"] = x.Method()

	m1 := make(VMStringMap)
	for k, v := range x.r.Header {
		if len(v) > 0 {
			m1[k] = VMString(v[0])
		}
	}
	rmap["Заголовки"] = m1

	m2 := make(VMStringMap)
	for k, v := range x.r.Form {
		if len(v) > 0 {
			m2[k] = VMString(v[0])
		}
	}
	rmap["Параметры"] = m2

	m3 := make(VMStringMap)
	for k, v := range x.r.PostForm {
		if len(v) > 0 {
			m3[k] = VMString(v[0])
		}
	}
	rmap["ПараметрыФормы"] = m3

	return rmap, nil
}

func (x *VMHttpRequest) MethodMember(name int) (VMFunc, bool) {

	// только эти методы будут доступны из кода на языке Гонец!

	switch names.UniqueNames.GetLowerCase(name) {
	case "метод":
		return VMFuncMustParams(0, x.Метод), true
	case "заголовок":
		return VMFuncMustParams(1, x.Заголовок), true
	case "установитьзаголовок":
		return VMFuncMustParams(2, x.УстановитьЗаголовок), true
	case "тело":
		return VMFuncMustParams(0, x.Тело), true
	case "путь":
		return VMFuncMustParams(0, x.Путь), true
	case "адрес":
		return VMFuncMustParams(0, x.Адрес), true
	case "фрагмент":
		return VMFuncMustParams(0, x.Фрагмент), true
	case "параметр":
		return VMFuncMustParams(1, x.Параметр), true
	case "данные":
		return VMFuncMustParams(0, x.Данные), true
	case "сообщение":
		return VMFuncMustParams(0, x.Сообщение), true
	}

	return nil, false
}

func (x *VMHttpRequest) Метод(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	rets.Append(x.Method())
	return nil
}

func (x *VMHttpRequest) Заголовок(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	s, ok := args[0].(VMString)
	if !ok {
		return VMErrorNeedString
	}
	rets.Append(x.GetHeader(s))
	return nil
}

func (x *VMHttpRequest) УстановитьЗаголовок(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	k, ok := args[0].(VMString)
	if !ok {
		return VMErrorNeedString
	}
	v, ok := args[1].(VMString)
	if !ok {
		return VMErrorNeedString
	}
	x.SetHeader(k, v)
	return nil
}

func (x *VMHttpRequest) Тело(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	s, _ := x.ReadBody()
	rets.Append(VMString(s))
	return nil
}

func (x *VMHttpRequest) Путь(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	rets.Append(x.Path())
	return nil
}

func (x *VMHttpRequest) Адрес(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	rets.Append(x.RemoteAddr())
	return nil
}

func (x *VMHttpRequest) Фрагмент(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	rets.Append(x.Fragment())
	return nil
}

func (x *VMHttpRequest) Параметр(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	s, ok := args[0].(VMString)
	if !ok {
		return VMErrorNeedString
	}
	rets.Append(x.GetParam(s))
	return nil
}

func (x *VMHttpRequest) Данные(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	rets.Append(x.data)
	return nil
}

func (x *VMHttpRequest) Сообщение(args VMSlice, rets *VMSlice, envout *(*Env)) error {
	v, err := x.RequestAsVMStringMap()
	if err != nil {
		return err
	}
	rets.Append(v)
	return nil
}

type VMHttpResponse struct {
	r    *http.Response
	w    http.ResponseWriter
	data VMValuer
}

func (x *VMHttpResponse) vmval() {}

func (x *VMHttpResponse) Interface() interface{} {
	return x.r
}

func (x *VMHttpResponse) String() string {
	return fmt.Sprintf("Ответ %s", x.r)
}

func (x *VMHttpResponse) MethodMember(name int) (VMFunc, bool) {

	// только эти методы будут доступны из кода на языке Гонец!

	switch names.UniqueNames.GetLowerCase(name) {
	// case "получить":
	// 	return VMFuncMustParams(0, x.Получить), true
	}

	return nil, false
}