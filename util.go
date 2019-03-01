package cvc

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

var (
	groupType reflect.Type
)

func init() {
	groupType = reflect.TypeOf((*Group)(nil)).Elem()
}

type StructMethod struct {
	Func reflect.Value
	Body reflect.Value
}

func (m StructMethod) Call(args ...reflect.Value) []reflect.Value {
	args = append(args[:0], append([]reflect.Value{m.Body}, args[0:]...)...)

	return m.Func.Call(args)
}

func (m StructMethod) Empty() bool {
	return m.Func == (reflect.Value{})
}

func (m StructMethod) NumIn() int {
	return m.Func.Type().NumIn()
}

func (m StructMethod) NumOut() int {
	return m.Func.Type().NumOut() - 1
}

func (m StructMethod) In(i int) reflect.Type {
	return m.Func.Type().In(i + 1)
}

func NormalizeVar(s string, sep string) string {
	if len(s) < 1 {
		return s
	}

	if len(sep) > 0 {
		re := regexp.MustCompile(regexp.QuoteMeta(sep))
		if re.FindIndex([]byte(s)) != nil {
			var r []string
			for _, d := range re.Split(s, -1) {
				r = append(r, NormalizeVar(d, ""))
			}

			return strings.Join(r, sep)
		}
	}

	rn := []rune(s)

	var n []rune
	n = append(n, rn[0])

	rem := rn[1:]
	for i, c := range rem {
		if i > 0 && unicode.IsUpper(c) && unicode.IsLower(rem[i-1]) {
			n = append(n, '-')
		}

		n = append(n, c)
	}

	return strings.ToLower(string(n))
}

func getConfigTypeByFuncs(fs ...StructMethod) string {
	var t string
	for _, f := range fs {
		t = getConfigTypeByFunc(f)
		if len(t) > 0 {
			break
		}
	}

	return t
}

func getConfigTypeByFunc(f StructMethod) string {
	kind := f.In(0).Kind()
	switch kind {
	case reflect.Bool:
		return "BoolVar"
	case reflect.Int:
		return "IntVar"
	case reflect.Int8:
		return "Int8Var"
	case reflect.Int16:
		return "Int16Var"
	case reflect.Int32:
		return "Int32Var"
	case reflect.Int64:
		return "Int64Var"
	case reflect.Uint:
		return "UintVar"
	case reflect.Uint8:
		return "Uint8Var"
	case reflect.Uint16:
		return "Uint16Var"
	case reflect.Uint32:
		return "Uint32Var"
	case reflect.Uint64:
		return "Uint64Var"
	case reflect.Float32:
		return "Float32Var"
	case reflect.Float64:
		return "Float64Var"
	case reflect.String:
		return "StringVar"
	default:
		return ""
	}
}

func getConfigTypeByValue(v reflect.Value) string {
	switch v.Interface().(type) {
	case bool:
		return "BoolVar"
	case int:
		return "IntVar"
	case int8:
		return "Int8Var"
	case int16:
		return "Int16Var"
	case int32:
		return "Int32Var"
	case int64:
		return "Int64Var"
	case uint:
		return "UintVar"
	case uint8:
		return "Uint8Var"
	case uint16:
		return "Uint16Var"
	case uint32:
		return "Uint32Var"
	case uint64:
		return "Uint64Var"
	case float32:
		return "Float32Var"
	case float64:
		return "Float64Var"
	case string:
		return "StringVar"
	default:
		switch v.Type().String() {
		case "time.Duration":
			return "DurationVar"
		}

		return "StringVar"
	}
}

func GetMethodByName(i interface{}, name string, numIn, numOut int) (m StructMethod, found bool) {
	var method reflect.Method
	method, found = reflect.TypeOf(i).MethodByName(name)
	if !found {
		return
	}
	if method.Func == (reflect.Value{}) {
		return
	}
	if !method.Func.IsValid() {
		return
	}

	m = StructMethod{Func: method.Func, Body: reflect.ValueOf(i)}
	if m.NumIn() != numIn || m.NumOut() != numOut {
		return
	}

	found = true
	return
}

func GetFuncFromItem(item *Item, name string, numIn, numOut int) []StructMethod {
	var fns []StructMethod

	var parseFunc StructMethod
	var found bool
	parseFunc, found = GetMethodByName(
		item.Value.Interface(),
		name,
		numIn,
		numOut,
	)
	if found && !parseFunc.Empty() {
		parseFunc.Body = item.Value
		fns = append(fns, parseFunc)
	}

	if item.Group != nil {
		parseFunc, found = GetMethodByName(
			item.Group.Value.Interface(),
			fmt.Sprintf("%s%s", name, item.FieldName),
			numIn,
			numOut,
		)
		if found && !parseFunc.Empty() {
			parseFunc.Body = item.Group.Value
			fns = append(fns, parseFunc)
		}
	}

	return fns
}

func GetFuncFromItemStruct(item *Item, name string, numIn, numOut int) []StructMethod {
	var fns []StructMethod

	var parseFunc StructMethod
	var found bool
	parseFunc, found = GetMethodByName(
		item.Value.Interface(),
		name,
		numIn,
		numOut,
	)
	if found && !parseFunc.Empty() {
		fns = append(fns, parseFunc)
	}

	if item.Group != nil {
		parseFunc, found = GetMethodByName(
			item.Group.Value.Interface(),
			fmt.Sprintf("%s%s", name, item.FieldName),
			numIn,
			numOut,
		)
		if found && !parseFunc.Empty() {
			fns = append(fns, parseFunc)
		}
	}

	return fns
}

func GetFlagValue(item *Item) (reflect.Value, error) {
	fns := GetFuncFromItem(item, "FlagValue", 1, 2)
	for _, fn := range fns {
		vs := fn.Call()
		return vs[0], nil
	}

	return item.Value, fmt.Errorf("not found `FlagValue` function")
}

func GetKeysFromViperConfig(group string, format string, v *viper.Viper, r io.Reader) ([]string, error) {
	nv := viper.New()
	nv.SetConfigType(format)
	if err := nv.ReadConfig(r); err != nil {
		return nil, err
	}

	var keys []string
	for _, k := range nv.AllKeys() {
		l := strings.SplitN(k, ".", 2)
		if len(l) != 2 {
			continue
		}
		if l[0] != group {
			continue
		}

		if i := v.Get(k); i == nil {
			return nil, fmt.Errorf("unknown key found: '%s'", k)
		}
		keys = append(keys, k)
	}

	return keys, nil
}

func getKeyFromConfig(prefix string, m map[string]interface{}) []string {
	var names []string
	for k, v := range m {
		n := k
		if len(prefix) > 0 {
			n = fmt.Sprintf("%s.%s", prefix, k)
		}

		switch v.(type) {
		case map[string]interface{}:
			names = append(names, getKeyFromConfig(n, v.(map[string]interface{}))...)
			continue
		case map[interface{}]interface{}:
			names = append(names, getKeyFromConfig(n, cast.ToStringMap(v))...)
			continue
		}

		names = append(names, n)
	}

	var filtered []string
	for _, i := range names {
		var found bool
		for _, f := range filtered {
			if i == f {
				found = true
				break
			}
		}

		if found {
			continue
		}
		filtered = append(filtered, i)
	}

	return filtered
}

func parseConfig(c interface{}) (*Item, map[string]*Item) {
	root := &Item{
		FieldName: "",
		Value:     reflect.ValueOf(c),
		Group:     nil,
		Tag:       "",
		IsGroup:   true,
	}

	return root, parseConfigField(c, reflect.TypeOf(c), root.Value, []*Item{root})
}

func parseConfigField(c interface{}, t reflect.Type, v reflect.Value, parents []*Item) map[string]*Item {
	group := parents[len(parents)-1]

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	m := map[string]*Item{}
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)

		var fv reflect.Value
		if v.Kind() == reflect.Ptr {
			fv = v.Elem().Field(i)
		} else {
			fv = v.Field(i)
		}

		if fv.Kind() == reflect.Ptr && fv.IsNil() {
			fv.Set(reflect.New(ft.Type.Elem()))
			fv = v.Elem().Field(i)
		}

		if ft.Type == reflect.TypeOf(BaseGroup{}) {
			continue
		} else if !fv.CanSet() {
			continue
		}

		item := &Item{
			FieldName: ft.Name,
			Value:     fv,
			Group:     group,
			Tag:       ft.Tag,
		}
		group.Children = append(group.Children, item)

		m[item.Name()] = item
		if ft.Type.Implements(groupType) {
			item.IsGroup = true
			n := parseConfigField(c, ft.Type, fv, append(parents, item))
			for k, v := range n {
				m[k] = v
			}
		}
	}

	return m
}

func CallParseFunc(f StructMethod, i interface{}) (interface{}, error) {
	rs := f.Call(reflect.ValueOf(i))
	v := rs[0].Interface()
	err := rs[1].Interface()
	if err == nil {
		return v, nil
	}

	e, found := err.(error)
	if found {
		return nil, e
	}

	log.Error(
		"ParseXX() return is not error type",
		"return", err,
		"kind", rs[1].Type().Kind(),
	)

	return nil, ErrorMethodNotFound
}

func CallValidateFunc(f StructMethod) error {
	rs := f.Call()
	err := rs[0].Interface()
	if err == nil {
		return nil
	}

	e, found := err.(error)
	if found {
		return e
	}

	log.Error(
		"ValidateXXX() return is not error type",
		"return", err,
		"kind", rs[0].Type().Kind(),
	)

	return ErrorMethodNotFound
}

func CallMergeFunc(f StructMethod) error {
	rs := f.Call()
	err := rs[0].Interface()
	if err == nil {
		return nil
	}

	e, found := err.(error)
	if found {
		return e
	}

	log.Error(
		"MergeXXX() return is not error type",
		"return", err,
		"kind", rs[0].Type().Kind(),
	)

	return ErrorMethodNotFound
}
