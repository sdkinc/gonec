package core

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"sync"

	"github.com/covrom/gonec/names"
)

type Vars struct {
	ids  []int      // сортированный массив идентификаторов из names.UniqueNames, отраженных в состав переменных
	vals []VMValuer // индекс значения равен индексу идентификатора в ids
}

func NewVars(cap int) *Vars {
	return &Vars{
		ids:  make([]int, 0, cap),
		vals: make([]VMValuer, 0, cap),
	}
}

func (v *Vars) Put(id int, val VMValuer, define bool) bool {
	i := sort.SearchInts(v.ids, id)
	if i == len(v.ids) {
		if define {
			v.ids = append(v.ids, id)
			v.vals = append(v.vals, val)
			return true
		}
	} else {
		if v.ids[i] == id {
			v.vals[i] = val
			return true
		} else if define {
			v.ids = append(v.ids, 0)
			v.vals = append(v.vals, VMNil)
			copy(v.ids[i+1:], v.ids[i:])
			v.ids[i] = id
			copy(v.vals[i+1:], v.vals[i:])
			v.vals[i] = val
			return true
		}
	}
	return false
}

func (v *Vars) Get(id int) (VMValuer, bool) {
	i := sort.SearchInts(v.ids, id)
	if i < len(v.ids) {
		if v.ids[i] == id {
			return v.vals[i], true
		}
	}
	return nil, false
}

func (v *Vars) Delete(id int) {
	i := sort.SearchInts(v.ids, id)
	if i < len(v.ids) {
		if v.ids[i] == id {
			copy(v.ids[i:], v.ids[i+1:])
			v.ids = v.ids[:len(v.ids)-1]
			copy(v.vals[i:], v.vals[i+1:])
			v.vals[len(v.vals)-1] = nil
			v.vals = v.vals[:len(v.vals)-1]
		}
	}
}

// Env provides interface to run VM. This mean function scope and blocked-scope.
// If stack goes to blocked-scope, it will make new Env.
type Env struct {
	sync.RWMutex
	name         string
	env          *Vars
	typ          map[int]reflect.Type
	parent       *Env
	interrupt    *bool
	stdout       io.Writer
	sid          string
	goRunned     bool
	lastid       int
	lastval      VMValuer
	builtsLoaded bool
}

func (e *Env) vmval() {} // нужно для того, чтобы *Env можно было сохранять в переменные VMValuer

// NewEnv creates new global scope.
// !!!не забывать вызывать core.LoadAllBuiltins(m)!!!
func NewEnv() *Env {
	b := false

	m := &Env{
		env:          NewVars(20),
		typ:          make(map[int]reflect.Type),
		parent:       nil,
		interrupt:    &b,
		stdout:       os.Stdout,
		goRunned:     false,
		lastid:       -1,
		builtsLoaded: false,
	}
	return m
}

// NewEnv создает новое окружение под глобальным контекстом переданного в e
func (e *Env) NewEnv() *Env {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.parent == nil {
			return &Env{
				env:          NewVars(20),
				typ:          make(map[int]reflect.Type),
				parent:       ee,
				interrupt:    e.interrupt,
				stdout:       e.stdout,
				goRunned:     false,
				lastid:       -1,
				builtsLoaded: ee.builtsLoaded,
			}

		}
	}
	panic("Не найден глобальный контекст!")
}

// NewSubEnv создает новое окружение под e, нужно для замыкания в анонимных функциях
func (e *Env) NewSubEnv() *Env {
	return &Env{
		env:          NewVars(20),
		typ:          make(map[int]reflect.Type),
		parent:       e,
		interrupt:    e.interrupt,
		stdout:       e.stdout,
		goRunned:     false,
		lastid:       -1,
		builtsLoaded: e.builtsLoaded,
	}
}

// Находим или создаем новый модуль в глобальном скоупе
func (e *Env) NewModule(n string) *Env {
	//ni := strings.ToLower(n)
	id := names.UniqueNames.Set(n)
	if v, err := e.Get(id); err == nil {
		if vv, ok := v.(*Env); ok {
			return vv
		}
	}

	m := e.NewEnv()
	m.name = n

	// на модуль можно ссылаться через переменную породившего глобального контекста
	e.DefineGlobal(id, m)
	return m
}

// func NewPackage(n string, w io.Writer) *Env {
// 	b := false

// 	return &Env{
// 		env:       make(map[string]reflect.Value),
// 		typ:       make(map[string]reflect.Type),
// 		parent:    nil,
// 		name:      strings.ToLower(n),
// 		interrupt: &b,
// 		stdout:    w,
// 	}
// }

func (e *Env) NewPackage(n string) *Env {
	return &Env{
		env:          NewVars(20),
		typ:          make(map[int]reflect.Type),
		parent:       e,
		name:         names.FastToLower(n),
		interrupt:    e.interrupt,
		stdout:       e.stdout,
		goRunned:     false,
		lastid:       -1,
		builtsLoaded: e.builtsLoaded,
	}
}

// Destroy deletes current scope.
func (e *Env) Destroy() {
	if e.parent == nil {
		return
	}

	if e.goRunned {
		e.Lock()
		defer e.Unlock()
		e.parent.Lock()
		defer e.parent.Unlock()
	}

	for k, v := range e.parent.env.vals {
		if vv, ok := v.(*Env); ok {
			if vv == e {
				e.parent.env.Delete(e.parent.env.ids[k])
			}
		}
	}
	e.parent = nil
	e.env = nil
}

func (e *Env) SetGoRunned(t bool) {
	for ee := e; ee != nil; ee = ee.parent {
		ee.goRunned = t
	}
}

func (e *Env) SetBuiltsIsLoaded() {
	e.builtsLoaded = true
}

func (e *Env) IsBuiltsLoaded() bool {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.builtsLoaded {
			return true
		}
	}
	return false
}

// SetName sets a name of the scope. This means that the scope is module.
func (e *Env) SetName(n string) {
	e.name = names.FastToLower(n)
}

// GetName returns module name.
func (e *Env) GetName() string {
	return e.name
}

// TypeName определяет имя типа по типу значения
func (e *Env) TypeName(t reflect.Type) int {

	for ee := e; ee != nil; ee = ee.parent {
		if ee.goRunned {
			ee.RLock()
		}
		for k, v := range ee.typ {
			if v == t {
				if ee.goRunned {
					ee.RUnlock()
				}
				return k
			}
		}
		if ee.goRunned {
			ee.RUnlock()
		}
	}
	return names.UniqueNames.Set(t.String())
}

// Type returns type which specified symbol. It goes to upper scope until
// found or returns error.
func (e *Env) Type(k int) (reflect.Type, error) {

	for ee := e; ee != nil; ee = ee.parent {
		if ee.goRunned {
			ee.RLock()
		}
		// if k >= 0 && k < len(ee.typ) {
		if v, ok := ee.typ[k]; ok {
			if ee.goRunned {
				ee.RUnlock()
			}
			return v, nil
		}
		// }
		if ee.goRunned {
			ee.RUnlock()
		}
		// if v, ok := ee.typ[k]; ok {
		// 	return v, nil
		// }
	}
	return nil, fmt.Errorf("Тип неопределен '%s'", names.UniqueNames.Get(k))
}

// Get returns value which specified symbol. It goes to upper scope until
// found or returns error.
func (e *Env) Get(k int) (VMValuer, error) {

	for ee := e; ee != nil; ee = ee.parent {
		if ee.goRunned {
			ee.RLock()
		}
		if ee.lastid == k {
			if ee.goRunned {
				ee.RUnlock()
			}
			return ee.lastval, nil
		}
		// if k >= 0 && k < len(ee.env) {
		// 	v := ee.env[k]
		// 	if v != nil {
		// 		if ee.goRunned {
		// 			ee.RUnlock()
		// 		}
		// 		return v, nil
		// 	}
		// }
		if v, ok := ee.env.Get(k); ok {
			if ee.goRunned {
				ee.RUnlock()
			}
			return v, nil
		}
		if ee.goRunned {
			ee.RUnlock()
		}
	}
	return nil, fmt.Errorf("Имя неопределено '%s'", names.UniqueNames.Get(k))
}

// Set modifies value which specified as symbol. It goes to upper scope until
// found or returns error.
func (e *Env) Set(k int, v VMValuer) error {

	for ee := e; ee != nil; ee = ee.parent {
		if ee.goRunned {
			ee.Lock()
		}
		if ok := ee.env.Put(k, v, false); ok {
			ee.lastid = k
			ee.lastval = v
			if ee.goRunned {
				ee.Unlock()
			}
			return nil
		}
		if ee.goRunned {
			ee.Unlock()
		}
	}
	return fmt.Errorf("Имя неопределено '%s'", names.UniqueNames.Get(k))
}

// DefineGlobal defines symbol in global scope.
func (e *Env) DefineGlobal(k int, v VMValuer) error {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.parent == nil {
			return ee.Define(k, v)
		}
	}
	return fmt.Errorf("Отсутствует глобальный контекст!")
}

// DefineType defines type which specifis symbol in global scope.
func (e *Env) DefineType(k int, t reflect.Type) error {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.parent == nil {
			if ee.goRunned {
				ee.Lock()
				defer ee.Unlock()
			}
			// for k >= len(ee.typ) {
			// 	ee.typ = append(ee.typ, nil)
			// }
			ee.typ[k] = t
			// // пишем в кэш индексы полей и методов для структур
			// // для работы со структурами нам нужен конкретный тип
			// if typ.Kind() == reflect.Ptr {
			// 	typ = typ.Elem()
			// }
			// if typ.Kind() == reflect.Struct {
			// 	// методы берем в т.ч. у ссылки на структуру, они включают методы самой структуры
			// 	// это будут разные методы для разных reflect.Value
			// 	ptyp := reflect.TypeOf(reflect.New(typ).Interface())
			// 	basicpath := typ.PkgPath() + "." + typ.Name() + "."

			// 	//методы
			// 	nm := typ.NumMethod()
			// 	for i := 0; i < nm; i++ {
			// 		meth := typ.Method(i)
			// 		// только экспортируемые
			// 		if meth.PkgPath == "" {
			// 			namtyp := UniqueNames.Set(basicpath + meth.Name)
			// 			// fmt.Println("SET METHOD: "+basicpath+meth.Name, meth.Index)
			// 			// ast.StructMethodIndexes.Cache[namtyp] = meth.Index
			// 		}
			// 	}
			// 	nm = ptyp.NumMethod()
			// 	for i := 0; i < nm; i++ {
			// 		meth := ptyp.Method(i)
			// 		// только экспортируемые
			// 		if meth.PkgPath == "" {
			// 			namtyp := UniqueNames.Set(basicpath + "*" + meth.Name)
			// 			// fmt.Println("SET *METHOD: "+basicpath+"*"+meth.Name, meth.Index)
			// 			// ast.StructMethodIndexes.Cache[namtyp] = meth.Index
			// 		}
			// 	}

			// 	//поля
			// 	nm = typ.NumField()
			// 	for i := 0; i < nm; i++ {
			// 		field := typ.Field(i)
			// 		// только экспортируемые неанонимные поля
			// 		if field.PkgPath == "" && !field.Anonymous {
			// 			namtyp := UniqueNames.Set(basicpath + field.Name)
			// 			// fmt.Println("SET FIELD: "+basicpath+field.Name, field.Index)
			// 			// ast.StructFieldIndexes.Cache[namtyp] = field.Index
			// 		}
			// 	}
			// }
			return nil
		}
	}
	return fmt.Errorf("Отсутствует глобальный контекст!")
}

func (e *Env) DefineTypeS(k string, t reflect.Type) error {
	return e.DefineType(names.UniqueNames.Set(k), t)
}

// DefineTypeStruct регистрирует системную функциональную структуру, переданную в виде указателя!
func (e *Env) DefineTypeStruct(k string, t interface{}) error {
	gob.Register(t)
	return e.DefineType(names.UniqueNames.Set(k), reflect.Indirect(reflect.ValueOf(t)).Type())
}

// Define defines symbol in current scope.
func (e *Env) Define(k int, v VMValuer) error {
	if e.goRunned {
		e.Lock()
	}
	// for k >= len(e.env) {
	// 	e.env = append(e.env, nil)
	// }
	e.env.Put(k, v, true)
	e.lastid = k
	e.lastval = v

	if e.goRunned {
		e.Unlock()
	}

	return nil
}

func (e *Env) DefineS(k string, v VMValuer) error {
	return e.Define(names.UniqueNames.Set(k), v)
}

// String return the name of current scope.
func (e *Env) String() string {
	return e.name
}

// Dump show symbol values in the scope.
func (e *Env) Dump() {
	if e.goRunned {
		e.RLock()
	}
	for i, k := range e.env.ids {
		e.Printf("%d %s = %#v %T\n", k, names.UniqueNames.Get(k), e.env.vals[i], e.env.vals[i])
	}
	if e.goRunned {
		e.RUnlock()
	}
}

func (e *Env) Println(a ...interface{}) (n int, err error) {
	// e.RLock()
	// defer e.RUnlock()
	return fmt.Fprintln(e.stdout, a...)
}

func (e *Env) Printf(format string, a ...interface{}) (n int, err error) {
	// e.RLock()
	// defer e.RUnlock()
	return fmt.Fprintf(e.stdout, format, a...)
}

func (e *Env) Sprintf(format string, a ...interface{}) string {
	// e.RLock()
	// defer e.RUnlock()
	return fmt.Sprintf(format, a...)
}

func (e *Env) Print(a ...interface{}) (n int, err error) {
	// e.RLock()
	// defer e.RUnlock()
	return fmt.Fprint(e.stdout, a...)
}

func (e *Env) StdOut() reflect.Value {
	// e.RLock()
	// defer e.RUnlock()
	return reflect.ValueOf(e.stdout)
}

func (e *Env) SetStdOut(w io.Writer) {
	// e.Lock()
	//пренебрегаем возможными коллизиями при установке потока вывода, т.к. это совсем редкая операция
	e.stdout = w
	// e.Unlock()
}

func (e *Env) SetSid(s string) error {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.parent == nil {
			ee.sid = s
			return ee.Define(names.UniqueNames.Set("ГлобальныйИдентификаторСессии"), VMString(s))
		}
	}
	return fmt.Errorf("Отсутствует глобальный контекст!")
}

func (e *Env) GetSid() string {
	for ee := e; ee != nil; ee = ee.parent {
		if ee.parent == nil {
			// пренебрегаем возможными коллизиями, т.к. изменение номера сессии - это совсем редкая операция
			return ee.sid
		}
	}
	return ""
}

func (e *Env) Interrupt() {
	*(e.interrupt) = true
}

func (e *Env) CheckInterrupt() bool {
	if *(e.interrupt) {
		*(e.interrupt) = false
		return true
	}
	return false
}
