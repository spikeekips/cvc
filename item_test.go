package cvc

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

type testItem struct {
	suite.Suite
}

func (t *testItem) TestName() {
	var value int = 10
	item := &Item{
		FieldName: "Test",
		Value:     reflect.ValueOf(value),
	}
	t.Equal("test", item.Name())
	t.Equal("test", item.FlagName())
}

func (t *testItem) TestComplexName() {
	var value int = 10
	item := &Item{
		FieldName: "TestShowme",
		Value:     reflect.ValueOf(value),
	}
	t.Equal("test_showme", item.Name())
	t.Equal("test_showme", item.FlagName())
}

func (t *testItem) TestFlagTag() {
	var value int = 10
	item := &Item{
		FieldName: "Test",
		Value:     reflect.ValueOf(value),
		Tag:       reflect.StructTag(`flag:"showme"`),
	}
	t.Equal("test", item.Name())
	t.Equal("showme", item.FlagName())
}

func (t *testItem) TestGroup() {
	group := &Item{
		FieldName: "ThisIsGroup",
		IsGroup:   true,
	}

	var value int = 10
	item := &Item{
		FieldName: "Test",
		Value:     reflect.ValueOf(value),
		Tag:       reflect.StructTag(`flag:"showme"`),
		Group:     group,
	}
	t.Equal("this_is_group.test", item.Name())
	t.Equal("this_is_group-showme", item.FlagName())
}

func (t *testItem) TestEnvName() {
	var value int = 10
	group := &Item{
		FieldName: "ThisIsGroup",
		IsGroup:   true,
	}

	item := &Item{
		FieldName: "TestFindMe",
		Value:     reflect.ValueOf(value),
		Group:     group,
	}

	t.Equal("THIS_IS_GROUP_TEST_FIND_ME", item.EnvName(""))

	item = &Item{
		FieldName: "Test-FindMe", // bad name
		Value:     reflect.ValueOf(value),
		Group:     group,
	}

	t.Empty(item.EnvName(""))
}

func (t *testItem) TestEnvNameFromTag() {
	var value int = 10
	group := &Item{
		FieldName: "ThisIsGroup",
		IsGroup:   true,
	}

	item := &Item{
		FieldName: "TestFindMe",
		Value:     reflect.ValueOf(value),
		Group:     group,
		Tag:       reflect.StructTag(`env:"KKKKK"`),
	}

	t.Equal("THIS_IS_GROUP_KKKKK", item.EnvName(""))
}

func TestItem(t *testing.T) {
	suite.Run(t, new(testItem))
}
