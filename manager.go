package cvc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	logging "github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type viperConfig struct {
	sync.Mutex
	format string
	r      *bytes.Reader
}

func (c viperConfig) Reader() io.Reader {
	c.Lock()
	defer c.Unlock()

	c.r.Seek(0, 0)
	b, _ := ioutil.ReadAll(c.r)
	return bytes.NewReader(b)
}

func (c viperConfig) Keys(group string, v *viper.Viper) ([]string, error) {
	return GetKeysFromViperConfig(group, c.format, v, c.Reader())
}

type Manager struct {
	sync.RWMutex
	name          string
	c             interface{}
	v             *viper.Viper
	cmd           *cobra.Command
	m             map[string]*Item
	fs            map[string]*Item
	root          *Item
	viperConfigs  []viperConfig
	envLookupFunc func(string) (string, bool)
	useEnv        bool
	group         string
	groups        []string
}

func NewManager(name string, c interface{}, cmd *cobra.Command, v *viper.Viper) *Manager {
	root, m := parseConfig(c)

	var groups []string
	thisCmd := cmd
	for {
		groups = append(groups, strings.Fields(thisCmd.Use)[0])
		thisCmd = thisCmd.Parent()
		if thisCmd == nil || thisCmd.Parent() == nil {
			break
		}
	}

	for i := len(groups)/2 - 1; i >= 0; i-- {
		opp := len(groups) - 1 - i
		groups[i], groups[opp] = groups[opp], groups[i]
	}

	group := strings.Join(groups, "-")

	if verbose {
		var envs []string
		for _, item := range m {
			envs = append(envs, item.EnvName(group))
		}

		log.Debug("available envs:", "env", envs)
	}

	fs := map[string]*Item{}
	for _, i := range m {
		fs[i.FlagName()] = i
	}

	manager := &Manager{
		name:          name,
		c:             c,
		cmd:           cmd,
		v:             v,
		m:             m,
		fs:            fs,
		root:          root,
		envLookupFunc: os.LookupEnv,
		useEnv:        true,
		group:         group,
		groups:        groups,
	}

	for _, item := range manager.Map() {
		manager.setFlag(item)
	}

	return manager
}

func (m *Manager) Merge() (string, error) {
	if m.UseEnv() {
		p, err := m.MergeFromEnv()
		if err != nil {
			return p, fmt.Errorf("failed to parse env, '%s': %v", p, err)
		}

	}

	{
		p, err := m.MergeFromViper()
		if err != nil {
			return p, fmt.Errorf("failed to parse viper, '%s': %v", p, err)
		}
	}

	{
		p, err := m.MergeFromFlags()
		if err != nil {
			return p, fmt.Errorf("failed to parse flag, '--%s': %v", p, err)
		}
	}

	if t, err := m.root.Validate(); err != nil {
		return t, fmt.Errorf("failed to validate, '%s': %v", t, err)
	}

	return "", nil
}

func (m *Manager) MergeFromEnv() (string, error) {
	m.Lock()
	defer m.Unlock()

	log_ := log.New(logging.Ctx{"type": "env"})
	log_.Debug("trying to merge")

	for _, item := range m.m {
		env := m.EnvName(item)
		input, found := m.envLookupFunc(env)
		if !found {
			continue
		}
		log.Debug("env found", "name", env, "value", input)
		v, err := item.ParseEnv(input)
		if err != nil {
			return env, err
		}

		if err := m.setRaw(item.Name(), v); err != nil {
			log_.Error("failed to merge", "env", env, "value", input, "error", err)
			return env, err
		}
	}

	return "", nil
}

func (m *Manager) MergeFromFlags() (string, error) {
	m.Lock()
	defer m.Unlock()

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

		item, found := m.itemByFlag(f.Name)
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
		if err := m.setRaw(item.Name(), a); err != nil {
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
	m.Lock()
	defer m.Unlock()

	log_ := log.New(logging.Ctx{"type": "viper"})

	log_.Debug("trying to merge")
	if len(m.viperConfigs) < 1 {
		log_.Debug("no config found; skip merging")
		return "", nil
	}

	var inserted []string
	for _, c := range m.viperConfigs {
		is, err := c.Keys(m.group, m.v)
		if err != nil {
			return "", err
		} else if len(is) < 1 {
			log_.Debug("no config values found")
			continue
		}
		log_.Debug("keys loaded", "keys", is)

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

		if err := m.v.ReadConfig(c.Reader()); err != nil {
			return "", err
		}
	}

	for _, k := range inserted {
		ks := strings.SplitN(k, ".", 2)
		if len(ks) < 2 {
			log_.Warn("unknown key found", "key", k)
			continue
		}
		if ks[0] != m.group {
			continue
		}
		key := ks[1]

		item, found := m.get(key)
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
		if err := m.setRaw(key, a); err != nil {
			log_.Error("failed to merge", "raw", k, "key", key, "value", m.v.Get(k), "error", err)
			return k, err
		}
		log_.Debug("item merged", "raw", k, "key", key, "value", a)
	}

	log_.Debug("merged")
	return "", nil
}

func (m *Manager) UseEnv() bool {
	m.RLock()
	defer m.RUnlock()

	return m.useEnv
}

func (m *Manager) Groups() []string {
	return m.groups
}

func (m *Manager) Group() string {
	return m.group
}

func (m *Manager) SetUseEnv(s bool) {
	m.Lock()
	defer m.Unlock()

	m.useEnv = s
}

func (m *Manager) SetViperConfig(format string, b []byte) error {
	m.Lock()
	defer m.Unlock()

	m.viperConfigs = append(m.viperConfigs, viperConfig{format: format, r: bytes.NewReader(b)})
	return nil
}

func (m *Manager) SetViperConfigFile(fs ...string) error {
	for _, f := range fs {
		if err := m.setViperConfigFile(f); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) setViperConfigFile(f string) error {
	ext := strings.ToLower(filepath.Ext(f))
	if len(ext) < 2 {
		return fmt.Errorf("no filename extension")
	}
	var found bool
	for _, e := range viper.SupportedExts {
		if e == ext[1:] {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unsupported file type found")
	}

	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}

	return m.SetViperConfig(ext[1:], b)
}

func (m *Manager) Root() *Item {
	m.RLock()
	defer m.RUnlock()

	return m.root
}

func (m *Manager) Config() interface{} {
	m.RLock()
	defer m.RUnlock()

	return m.c
}

func (m *Manager) Envs() []string {
	m.RLock()
	defer m.RUnlock()

	var envs []string
	for _, item := range m.m {
		if item.IsGroup {
			continue
		}
		envs = append(envs, m.EnvName(item))
	}

	sort.Strings(envs)
	return envs
}

func (m *Manager) EnvName(item *Item) string {
	prefix := m.group
	if len(m.name) > 0 {
		prefix = m.name + "-" + prefix
	}

	return item.EnvName(prefix)
}

func (m *Manager) ConfigPprint() (o []interface{}) {
	m.RLock()
	defer m.RUnlock()

	for _, item := range m.m {
		if item.IsGroup {
			continue
		}
		o = append(o, "config-"+item.Name(), item.Value.Interface())
	}
	return
}

func (m *Manager) ConfigString() string {
	m.RLock()
	defer m.RUnlock()

	b, err := json.MarshalIndent(m.c, "", "  ")
	if err != nil {
		log.Error("failed to marshal config", "error", err)
		return ""
	}

	return string(b)
}

func (m *Manager) Cobra() *cobra.Command {
	return m.cmd
}

func (m *Manager) FlagSet() *pflag.FlagSet {
	m.RLock()
	defer m.RUnlock()

	return m.cmd.Flags()
}

func (m *Manager) Viper() *viper.Viper {
	m.RLock()
	defer m.RUnlock()

	return m.v
}

func (m *Manager) ViperString(format string) (string, error) {
	m.RLock()
	defer m.RUnlock()

	f, err := ioutil.TempFile("", fmt.Sprintf("cvc*.%s", format))
	if err != nil {
		return "", err
	}
	defer func() {
		os.Remove(f.Name())
	}()

	if err = m.v.WriteConfigAs(f.Name()); err != nil {
		return "", err
	}

	b, err := ioutil.ReadFile(f.Name())
	if err != nil {
		return "", err
	}

	if format == "props" || format == "properties" {
		var lines []string
		bf := bufio.NewReader(bytes.NewBuffer(b))
		for {
			l, err := bf.ReadString('\n')
			if err == io.EOF {
				break
			}
			s := strings.TrimSpace(l)
			if len(s) < 1 {
				continue
			}

			lines = append(lines, s)
		}

		sort.Strings(lines)

		return strings.Join(lines, "\n"), nil
	}

	return strings.TrimSpace(string(b)), nil
}

func (m *Manager) Map() map[string]*Item {
	m.RLock()
	defer m.RUnlock()

	return m.m
}

func (m *Manager) Get(key string) (*Item, bool) {
	m.RLock()
	defer m.RUnlock()

	return m.get(key)
}

func (m *Manager) get(key string) (*Item, bool) {
	c, found := m.m[key]
	return c, found
}

func (m *Manager) GetValue(key string, i interface{}) error {
	m.RLock()
	defer m.RUnlock()

	return m.getValue(key, i)
}

func (m *Manager) getValue(key string, i interface{}) error {
	item, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found")
	}

	reflect.ValueOf(i).Elem().Set(item.Value)
	return nil
}

func (m *Manager) SetValue(key string, i interface{}) error {
	m.Lock()
	defer m.Unlock()

	return m.setValue(key, i)
}

func (m *Manager) setValue(key string, i interface{}) error {
	item, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found: '%v'", key)
	}

	r, err := item.Parse(i)
	if err != nil {
		return err
	}

	if !item.Value.Type().AssignableTo(reflect.TypeOf(r)) {
		return fmt.Errorf("not assignable")
	}

	item.Value.Set(reflect.ValueOf(r))
	return nil
}

func (m *Manager) SetRaw(key string, i interface{}) error {
	m.Lock()
	defer m.Unlock()

	return m.setRaw(key, i)
}

func (m *Manager) setRaw(key string, i interface{}) error {
	item, found := m.m[key]
	if !found {
		return fmt.Errorf("key not found: '%v'", key)
	}

	if !item.Value.CanSet() {
		return fmt.Errorf("cannot set")
	}

	if !item.Value.Type().AssignableTo(reflect.TypeOf(i)) {
		return fmt.Errorf("not assignable: %T - %T", item.Value.Interface(), i)
	}

	item.Value.Set(reflect.ValueOf(i))
	return nil
}

func (m *Manager) SetEnvLookupFunc(fn func(string) (string, bool)) {
	m.Lock()
	defer m.Unlock()

	m.envLookupFunc = fn
}

func (m *Manager) ItemByFlag(flag string) (*Item, bool) {
	m.RLock()
	defer m.RUnlock()

	return m.itemByFlag(flag)
}

func (m *Manager) itemByFlag(flag string) (*Item, bool) {
	c, found := m.fs[flag]
	return c, found
}

func (m *Manager) setFlag(item *Item) error {
	if item.IsGroup {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	if len(item.FlagName()) < 1 {
		return nil
	}

	call := func(t string, v interface{}, d reflect.Value) {
		if !item.EnableFlag() {
			return
		}

		method, _ := GetMethodByName(m.cmd.Flags(), t, 4, 0)
		method.Call(
			reflect.ValueOf(v),
			reflect.ValueOf(item.FlagName()),
			d,
			reflect.ValueOf(item.Tag.Get("flag-help")),
		)
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
			vs := method.Call()
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
	case "DurationVar":
		var b *time.Duration = new(time.Duration)
		call(t, b, defaultValue)
		item.Input = b
	default:
		return fmt.Errorf("value type, '%s' is not supported by flag", t)
	}

	viperName := m.group + "." + item.Name()
	m.v.SetDefault(viperName, defaultValue.Interface())

	item.ViperName = viperName

	return nil
}
