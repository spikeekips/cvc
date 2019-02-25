package cvc

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

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
			n = append(n, '_')
		}

		n = append(n, c)
	}

	return strings.ToLower(string(n))
}

func getConfigTypeByFuncs(fs ...reflect.Value) string {
	var t string
	for _, f := range fs {
		t = getConfigTypeByFunc(f)
		if len(t) > 0 {
			break
		}
	}

	return t
}

func getConfigTypeByFunc(f reflect.Value) string {
	kind := f.Type().In(0).Kind()
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
		return "StringVar"
	}
}

func GetMethodByName(i interface{}, name string, numIn, numOut int) (m reflect.Value, found bool) {
	m = reflect.ValueOf(i).MethodByName(name)
	if m == (reflect.Value{}) {
		return
	}
	if !m.IsValid() {
		return
	}

	if m.Type().NumIn() != numIn || m.Type().NumOut() != numOut {
		return
	}

	found = true
	return
}

func GetFuncFromItem(item *Item, name string, numIn, numOut int) []reflect.Value {
	var fns []reflect.Value

	var parseFunc reflect.Value
	var found bool
	parseFunc, found = GetMethodByName(
		item.Value.Interface(),
		name,
		numIn,
		numOut,
	)
	if found && parseFunc != (reflect.Value{}) {
		fns = append(fns, parseFunc)
	}

	if item.Group != nil {
		parseFunc, found = GetMethodByName(
			item.Group.Value.Interface(),
			fmt.Sprintf("%s%s", name, item.FieldName),
			numIn,
			numOut,
		)
		if found && parseFunc != (reflect.Value{}) {
			fns = append(fns, parseFunc)
		}
	}

	return fns
}

func GetFlagValue(item *Item) (reflect.Value, error) {
	fns := GetFuncFromItem(item, "FlagValue", 1, 2)
	for _, fn := range fns {
		vs := fn.Call([]reflect.Value{})
		return vs[0], nil
	}

	return item.Value, fmt.Errorf("not found `FlagValue` function")
}

func GetKeysFromViperConfig(v *viper.Viper, r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b = []byte(strings.TrimSpace(string(b)))

	var m map[string]interface{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	names := getKeyFromConfig("", m)
	for _, k := range names {
		if i := v.Get(k); i == nil {
			return nil, fmt.Errorf("unknown key found: '%s'", k)
		}
	}

	return names, nil
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
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(string(debug.Stack()))
		}
	}()

	root := &Item{
		FieldName: "",
		Value:     reflect.ValueOf(c),
		Group:     nil,
		Tag:       "",
		IsGroup:   true,
	}

	return root, parseConfigField(c, root.Value, []*Item{root})
}

func parseConfigField(c interface{}, t reflect.Value, parents []*Item) map[string]*Item {
	group := parents[len(parents)-1]

	m := map[string]*Item{}
	for i := 0; i < t.Elem().NumField(); i++ {
		fv := t.Elem().Field(i)
		ft := t.Elem().Type().Field(i)

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
			n := parseConfigField(c, fv.Elem(), append(parents, item))
			for k, v := range n {
				m[k] = v
			}
		}
	}

	return m
}

func CallParseFunc(f reflect.Value, i interface{}) (interface{}, error) {
	rs := f.Call([]reflect.Value{reflect.ValueOf(i)})
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

func CallValidateFunc(f reflect.Value) error {
	rs := f.Call([]reflect.Value{})
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