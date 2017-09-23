package core

import (
	"reflect"

	"github.com/covrom/gonec/names"
)

// VMMetaObj корневая структура для системных функциональных структур Го, доступных из языка Гонец
// поля и методы должны отличаться друг от друга без учета регистра
// например, Set и set - в вирт. машине будут считаться одинаковыми, будет использоваться последнее по индексу
type VMMetaObj struct {
	vmMetaCacheM map[int]VMFunc
	vmMetaCacheF map[int][]int
	vmOriginal   VMMetaObject
}

func (v *VMMetaObj) vmval() {}

func (v *VMMetaObj) Interface() interface{} {
	// возвращает ссылку на структуру, от которой был вызван метод VMCacheMembers
	//rv:=*(*VMMetaObject)(v.vmOriginal)
	return v.vmOriginal
}

// VMCacheMembers кэширует все поля и методы ссылки на объединенную структуру в VMMetaObject
// Вызывать эту функцию надо так:
// v:=&struct{VMMetaObj, a int}{}; v.VMCacheMembers(v)
func (v *VMMetaObj) VMCacheMembers(vv VMMetaObject) {
	v.vmMetaCacheM = make(map[int]VMFunc)
	v.vmMetaCacheF = make(map[int][]int)
	v.vmOriginal = vv

	rv := reflect.ValueOf(vv)
	typ := reflect.TypeOf(vv)

	// пишем в кэш индексы полей и методов для структур

	// методы
	nm := typ.NumMethod()
	for i := 0; i < nm; i++ {
		meth := typ.Method(i)
		// только экспортируемые
		if meth.PkgPath == "" {
			namtyp := names.UniqueNames.Set(meth.Name)
			v.vmMetaCacheM[namtyp] = func(vmeth reflect.Value) VMFunc {
				// TODO: сделать бенчмарк вызова функций
				return VMFunc(
					func(args VMSlice, rets *VMSlice) error {
						x := make([]reflect.Value, len(args))
						for i := range args {
							x[i] = reflect.ValueOf(args[i])
						}
						r := vmeth.Call(x)
						switch len(r) {
						case 0:
							return nil
						case 1:
							if x, ok := r[0].Interface().(VMValuer); ok {
								rets.Append(x)
								return nil
							}
							rets.Append(ReflectToVMValue(r[0]))
							return nil
						case 2:
							if e, ok := r[1].Interface().(error); ok {
								rets.Append(ReflectToVMValue(r[0]))
								return e
							}
							fallthrough
						default:
							*rets = make(VMSlice, len(r))
							for i := range r {
								(*rets)[i] = ReflectToVMValue(r[i])
							}
							return nil
						}
					})
			}(rv.Method(meth.Index))
		}
	}

	// поля
	nm = typ.NumField()
	for i := 0; i < nm; i++ {
		field := typ.Field(i)
		// только экспортируемые неанонимные поля
		if field.PkgPath == "" && !field.Anonymous {
			namtyp := names.UniqueNames.Set(field.Name)
			v.vmMetaCacheF[namtyp] = field.Index
		}
	}
}

func (v *VMMetaObj) VMIsField(name int) bool {
	_, ok := v.vmMetaCacheF[name]
	return ok
}

func (v *VMMetaObj) VMGetField(name int) VMInterfacer {
	vv := reflect.ValueOf(v.Interface()).FieldByIndex(v.vmMetaCacheF[name])
	// поля с типом вирт. машины вернем сразу
	if x, ok := vv.Interface().(VMInterfacer); ok {
		return x
	}
	return ReflectToVMValue(vv)
}

func (v *VMMetaObj) VMSetField(name int, val VMInterfacer) {
	vv := reflect.ValueOf(v.Interface()).FieldByIndex(v.vmMetaCacheF[name])
	if !vv.CanSet() {
		panic("Невозможно установить значение поля только для чтения")
	}
	// поля с типом вирт. машины присваиваем без конверсии
	if _, ok := vv.Interface().(VMInterfacer); ok {
		vv.Set(reflect.ValueOf(val))
		return
	}
	if reflect.TypeOf(val).AssignableTo(vv.Type()) {
		vv.Set(reflect.ValueOf(val.Interface()))
		return
	}
	if reflect.TypeOf(val).ConvertibleTo(vv.Type()) {
		vv.Set(reflect.ValueOf(val).Convert(vv.Type()))
		return
	}
	panic("Невозможно установить значение поля")
}

// VMGetMethod генерит функцию,
// которая возвращает либо одно значение и ошибку, либо массив значений интерпретатора VMSlice
func (v *VMMetaObj) VMGetMethod(name int) (VMFunc, bool) {
	rv, ok := v.vmMetaCacheM[name]
	return rv, ok
}