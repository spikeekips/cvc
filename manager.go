package cvc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	logging "github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	groupType reflect.Type
)

func init() {
	groupType = reflect.TypeOf((*Group)(nil)).Elem()
}

type Manager struct {
	c             interface{}
	v             *viper.Viper
	cmd           *cobra.Command
	m             map[string]*Item
	fs            map[string]*Item
	root          *Item
	viperConfigs  []*bytes.Reader
	envLookupFunc func(string) (string, bool)
	useEnv        bool
}

func NewManager(c interface{}, cmd *cobra.Command, v *viper.Viper) *Manager {
	root, m := parseConfig(c)

	fs := map[string]*Item{}
	for _, i := range m {
		fs[i.FlagName()] = i
	}

	manager := &Manager{
		c:             c,
		cmd:           cmd,
		v:             v,
		m:             m,
		fs:            fs,
		root:          root,
		envLookupFunc: os.LookupEnv,
		useEnv:        true,
	}

	for _, item := range manager.Map() {
		manager.setFlag(item)
	}

	return manager
}

func (m *Manager) Merge() (string, error) {
	if m.useEnv {
		p, err := m.MergeFromEnv()
		if err != nil {
			return p, err
		}
	}

	{
		p, err := m.MergeFromViper()
		if err != nil {
			return p, err
		}
	}

	{
		p, err := m.MergeFromFlags()
		if err != nil {
			return p, err
		}
	}

	if t, err := m.root.Validate(); err != nil {
		return t, err
	}

	return "", nil
}

func (m *Manager) MergeFromEnv() (string, error) {
	log_ := log.New(logging.Ctx{"type": "env"})
	log_.Debug("trying to merge")

	prefix := strings.Fields(m.cmd.Use)[0]

	for _, item := range m.m {
		env := item.EnvName(prefix)
		input, found := m.envLookupFunc(env)
		if !found {
			continue
		}
		log.Debug("env found", "name", env, "value", input)
		v, err := item.ParseEnv(input)
		if err != nil {
			return env, err
		}

		if err := m.SetRaw(item.Name(), v); err != nil {
			log_.Error("failed to merge", "env", env, "value", input, "error", err)
			return env, err
		}
	}

	return "", nil
}

func (m *Manager) MergeFromFlags() (string, error) {
	log_ := log.New(logging.Ctx{"type": "flag"})
	log_.Debug("trying to merge")

	var err error
	var problemFlag string
	m.cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if err != nil {
			return
		}

		if !f.Changed {
			return
		}

		item, found := m.ItemByFlag(f.Name)
		if !found {
			problemFlag = f.Name
			err = fmt.Errorf("unknown flag found: '%s'", f.Name)
			return
		}

		input := reflect.ValueOf(item.Input).Elem().Interface()

		var a interface{}
		a, err = item.Parse(input)
		log_.Debug("parsed", "flag", f.Name, "value", input, "error", err)
		if err != nil {
			problemFlag = f.Name
			return
		}
		if err := m.SetRaw(item.Name(), a); err != nil {
			problemFlag = f.Name
			log_.Error("failed to merge", "flag", f.Name, "value", input, "error", err)
			return
		}
		log_.Debug("item merged", "flag", f.Name, "value", input)
	})

	log_.Debug("merged")
	return problemFlag, err
}

func (m *Manager) MergeFromViper() (string, error) {
	log_ := log.New(logging.Ctx{"type": "viper"})

	log_.Debug("trying to merge")
	if len(m.viperConfigs) < 1 {
		log_.Debug("no config found; skip merging")
		return "", nil
	}

	var inserted []string
	for _, r := range m.viperConfigs {
		r.Seek(0, 0)
		is, err := GetKeysFromViperConfig(m.Viper(), r)
		if err != nil {
			return "", err
		} else if len(is) < 1 {
			log_.Debug("no config values found")
			continue
		}

		for _, i := range is {
			var found bool
			for _, j := range inserted {
				if i == j {
					found = true
					break
				}
			}
			if found {
				continue
			}
			inserted = append(inserted, i)
		}

		r.Seek(0, 0)
		if err := m.v.ReadConfig(r); err != nil {
			return "", err
		}
	}

	group := strings.Fields(m.cmd.Use)[0]

	for _, k := range inserted {
		ks := strings.SplitN(k, ".", 2)
		if len(ks) < 2 {
			log_.Warn("unknown key found", "key", k)
			continue
		}
		if ks[0] != group {
			continue
		}
		key := ks[1]

		item, found := m.Get(key)
		if !found {
			log_.Warn("unknown key found", "raw", k, "key", key)
			continue
		}

		a, err := item.Parse(m.v.Get(k))
		log_.Debug("parsed", "key", k, "value", m.v.Get(k), "error", err)
		if err != nil {
			log_.Error("failed to parse", "raw", k, "key", key, "error", err, "input", m.v.Get(k))
			return k, err
		}
		if err := m.SetRaw(key, a); err != nil {
			log_.Error("failed to merge", "raw", k, "key", key, "value", m.v.Get(k), "error", err)
			return k, err
		}
		log_.Debug("item merged", "raw", k, "key", key, "value", a)
	}

	log_.Debug("merged")
	return "", nil
}

func (m *Manager) UseEnv(s bool) {
	m.useEnv = s
}

func (m *Manager) SetViperConfig(b []byte) error {
	m.viperConfigs = append(m.viperConfigs, bytes.NewReader(b))
	return nil
}

func (m *Manager) SetViperConfigFile(f string) error {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	return m.SetViperConfig(b)
}

func (m *Manager) Root() *Item {
	return m.root
}

func (m *Manager) Config() interface{} {
	return m.c
}

func (m *Manager) Envs() []string {
	prefix := strings.Fields(m.cmd.Use)[0]

	var envs []string
	for _, item := range m.m {
		envs = append(envs, item.EnvName(prefix))
	}

	return envs
}

func (m *Manager) ConfigPprint() (o []interface{}) {
	for _, item := range m.m {
		if item.IsGroup {
			continue
		}
		o = append(o, "config-"+item.Name(), item.Value.Interface())
	}
	return
}

func (m *Manager) ConfigString() string {
	b, err := json.MarshalIndent(m.c, "", "  ")
	if err != nil {
		log.Error("failed to marshal config", "error", err)
		return ""
	}

	return string(b)
}

func (m *Manager) FlagSet() *pflag.FlagSet {
	return m.cmd.Flags()
}

func (m *Manager) Viper() *viper.Viper {
	return m.v
}

func (m *Manager) Map() map[string]*Item {
	return m.m
}

func (m *Manager) Get(key string) (*Item, bool) {
	c, found := m.m[key]
	return c, found
}

func (m *Manager) GetValue(key string, i interface{}) error {
	k, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found")
	}

	reflect.ValueOf(i).Elem().Set(k.Value)
	return nil
}

func (m *Manager) SetValue(key string, i interface{}) error {
	k, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found: '%v'", key)
	}

	r, err := k.Parse(i)
	if err != nil {
		return err
	}

	if !k.Value.Type().AssignableTo(reflect.TypeOf(r)) {
		return fmt.Errorf("not assignable")
	}

	k.Value.Set(reflect.ValueOf(r))
	return nil
}

func (m *Manager) SetRaw(key string, i interface{}) error {
	k, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found: '%v'", key)
	}

	if !k.Value.CanSet() {
		return fmt.Errorf("cannot set")
	}

	if !k.Value.Type().AssignableTo(reflect.TypeOf(i)) {
		return fmt.Errorf("not assignable: %T - %T", k.Value.Interface(), i)
	}

	k.Value.Set(reflect.ValueOf(i))
	return nil
}

func (m *Manager) SetEnvLookupFunc(fn func(string) (string, bool)) {
	m.envLookupFunc = fn
}

func (m *Manager) ItemByFlag(flag string) (*Item, bool) {
	c, found := m.fs[flag]
	return c, found
}

func (m *Manager) setFlag(item *Item) error {
	if len(item.FlagName()) < 1 {
		return nil
	}

	call := func(t string, v interface{}, d reflect.Value) {
		method, _ := GetMethodByName(m.cmd.Flags(), t, 4, 0)
		method.Call([]reflect.Value{
			reflect.ValueOf(v),
			reflect.ValueOf(item.FlagName()),
			d,
			reflect.ValueOf(item.Tag.Get("flag-help")),
		})
		return
	}

	fns := GetFuncFromItem(item, "Parse", 1, 2)
	t := getConfigTypeByFuncs(fns...)

	if len(t) < 1 {
		t = getConfigTypeByValue(item.Value)
	}

	var defaultValue reflect.Value = item.Value
	if d, err := GetFlagValue(item); err == nil {
		defaultValue = d
	} else if t == "StringVar" {
		method, found := GetMethodByName(item.Value.Interface(), "StringVar", 0, 1)
		if !found {
			defaultValue = reflect.ValueOf(fmt.Sprintf("%v", item.Value.Interface()))
		} else {
			vs := method.Call([]reflect.Value{})
			if len(vs) > 0 {
				defaultValue = vs[0]
			}
		}
	}

	switch t {
	case "BoolVar":
		var b *bool = new(bool)
		call(t, b, defaultValue)
		item.Input = b
	case "IntVar":
		var b *int = new(int)
		call(t, b, defaultValue)
		item.Input = b
	case "Int8Var":
		var b *int8 = new(int8)
		call(t, b, defaultValue)
		item.Input = b
	case "Int16Var":
		var b *int16 = new(int16)
		call(t, b, defaultValue)
		item.Input = b
	case "Int32Var":
		var b *int32 = new(int32)
		call(t, b, defaultValue)
		item.Input = b
	case "Int64Var":
		var b *int64 = new(int64)
		call(t, b, defaultValue)
		item.Input = b
	case "UintVar":
		var b *uint = new(uint)
		call(t, b, defaultValue)
		item.Input = b
	case "Uint8Var":
		var b *uint8 = new(uint8)
		call(t, b, defaultValue)
		item.Input = b
	case "Uint16Var":
		var b *uint16 = new(uint16)
		call(t, b, defaultValue)
		item.Input = b
	case "Uint32Var":
		var b *uint32 = new(uint32)
		call(t, b, defaultValue)
		item.Input = b
	case "Uint64Var":
		var b *uint64 = new(uint64)
		call(t, b, defaultValue)
		item.Input = b
	case "Float32Var":
		var b *float32 = new(float32)
		call(t, b, defaultValue)
		item.Input = b
	case "Float64Var":
		var b *float64 = new(float64)
		call(t, b, defaultValue)
		item.Input = b
	case "StringVar":
		var b *string = new(string)
		call(t, b, defaultValue)
		item.Input = b
	default:
		return fmt.Errorf("value type, '%s' is not supported by flag", t)
	}

	group := strings.Fields(m.cmd.Use)[0]

	viperName := group + "." + item.Name()
	m.v.SetDefault(viperName, defaultValue.Interface())

	item.ViperName = viperName

	return nil
}
