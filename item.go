package cvc

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	logging "github.com/inconshreveable/log15"
)

var (
	// regexpEnvName should be consisted by uppercase letters, digits, and the '_)
	regexpEnvName *regexp.Regexp = regexp.MustCompile("(?i)^[a-z_][a-z0-9_]*$")
)

type Group interface {
	ThisIsGroup()
}

type BaseGroup struct{}

func (b *BaseGroup) Validate() error {
	return nil
}

type Item struct {
	FieldName string
	Value     reflect.Value
	Group     *Item
	Children  []*Item
	Tag       reflect.StructTag
	Input     interface{}
	IsGroup   bool
	ViperName string
}

func (c Item) String() string {
	groupName := ""
	if c.Group != nil {
		groupName = c.Group.Name()
	}
	return fmt.Sprintf(
		"<Item Name()=%s FieldName=%s Group=%s Children=%d IsGroup=%v>",
		c.Name(),
		c.FieldName,
		groupName,
		len(c.Children),
		c.IsGroup,
	)
}

func (c *Item) prefixes() []string {
	var names []string

	var group *Item = c.Group
	for {
		if group == nil {
			break
		}
		if len(group.FieldName) < 1 {
			break
		}
		names = append(names[:0], append([]string{group.FieldName}, names[0:]...)...)

		group = group.Group
	}

	return names
}

func (c *Item) FlagName() string {
	tag := c.Tag.Get("flag")
	switch {
	case tag == "-":
		return ""
	case len(tag) > 0:
		return strings.Replace(c.name(tag), ".", "-", -1)
	default:
		return strings.Replace(c.Name(), ".", "-", -1)
	}

	return ""
}

func (c *Item) EnvName(prefix string) string {
	var t string
	env := c.Tag.Get("env")
	switch {
	case env == "-":
		return ""
	case len(env) > 0:
		t = env
	default:
		t = c.FieldName
	}

	s := strings.ToUpper(NormalizeVar(
		strings.Join(append(c.prefixes(), t), "_"),
		"_",
	))
	if !regexpEnvName.MatchString(s) {
		log.Error("invalid env name found", "name", s)
		return ""
	}

	if len(prefix) < 1 {
		return s
	}
	return strings.ToUpper(prefix) + "_" + s
}

func (c *Item) name(n string) string {
	return NormalizeVar(
		strings.Join(append(c.prefixes(), n), "."),
		".",
	)
}

func (c *Item) Name() string {
	return c.name(c.FieldName)
}

func (c *Item) Validate() (string, error) {
	for _, c := range c.Children {
		if n, err := c.Validate(); err != nil {
			return n, err
		}
	}

	if err := c.validate(); err != nil {
		return c.Name(), err
	}

	return "", nil
}

func (c *Item) validate() error {
	fns := GetFuncFromItem(c, "Validate", 0, 1)
	for _, f := range fns {
		return CallValidateFunc(f)
	}

	return nil
}

func (c *Item) Parse(i interface{}) (interface{}, error) {
	fns := GetFuncFromItem(c, "Parse", 1, 2)
	for _, f := range fns {
		return CallParseFunc(f, i)
	}

	return i, nil
}

func (c *Item) ParseEnv(i string) (interface{}, error) {
	log_ := log.New(logging.Ctx{"item": c.Name(), "action": "parseEnv", "input": i})

	fns := GetFuncFromItem(c, "ParseEnv", 1, 2)
	for _, f := range fns {
		return CallParseFunc(f, i)
	}

	fns = GetFuncFromItem(c, "Parse", 1, 2)
	t := getConfigTypeByFuncs(fns...)

	if len(t) < 1 {
		t = getConfigTypeByValue(c.Value)
	}

	switch t {
	case "StringVar":
		return c.Parse(i)
	default:
		log_.Error("failed to parse", "type", t)
		return nil, fmt.Errorf("failed to parse env value")
	}

	return nil, nil
}
