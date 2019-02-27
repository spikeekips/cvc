package cvc

import (
	"io/ioutil"
	"sort"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

type testManager struct {
	suite.Suite
}

func (t *testManager) TestNew() {
	config := &struct {
		A int
		B string
	}{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	vp := viper.New()

	manager := NewManager(config, cmd, vp)
	t.NotEmpty(manager.Config())
}

func (t *testManager) TestRoot() {
	config := &struct {
		A int
		B string
	}{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	vp := viper.New()

	manager := NewManager(config, cmd, vp)
	t.NotEmpty(manager.Root())
	t.Equal(2, len(manager.Root().Children))

	{
		it, found := manager.Get("a")
		t.True(found)
		t.NotEmpty(it.Group)
		t.False(it.IsGroup)
	}
	{
		it, found := manager.Get("b")
		t.True(found)
		t.NotEmpty(it.Group)
		t.False(it.IsGroup)
	}
}

func (t *testManager) TestMerge() {
	config := &struct {
		A int
		B string
	}{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)

	cmd.SetArgs([]string{"--a", "10", "--b", "20"})
	err := cmd.Execute()
	t.NoError(err)

	key, err := manager.Merge()
	t.Empty(key)
	t.NoError(err)

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(10, merged)
	}
	{
		var merged string
		err := manager.GetValue("b", &merged)
		t.NoError(err)
		t.Equal("20", merged)
	}
}

type testConfig struct {
	A int
	B string

	parseA   func(string) (int, error)
	validate func() error
}

func (t *testConfig) ParseA(i string) (int, error) {
	if t.parseA != nil {
		return t.parseA(i)
	}

	n, err := strconv.ParseInt(i, 10, 64)
	return int(n), err
}

func (t *testConfig) Validate() error {
	if t.validate != nil {
		return t.validate()
	}

	return nil
}

func (t *testManager) TestParse() {
	config := &testConfig{
		A: 1,
		B: "2",
		parseA: func(i string) (int, error) {
			return 1000, nil
		},
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)

	cmd.SetArgs([]string{"--a", "10", "--b", "20"})
	err := cmd.Execute()
	t.NoError(err)

	manager.Merge()

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(1000, merged)
	}
}

func (t *testManager) TestValidate() {
	config := &testConfig{
		A: 1,
		B: "2",
	}
	config.validate = func() error {
		config.A = 200
		return nil
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)
	manager.Merge()

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(200, merged)
	}
}

func (t *testManager) TestViper() {
	config := &testConfig{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()
	vp.SetConfigType("yml")

	manager := NewManager(config, cmd, vp)
	manager.SetViperConfig("yml", []byte(`
naru:
  a: "10"
  b: "2"
`))
	_, err := manager.Merge()
	t.NoError(err)

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(10, merged)
	}
}

func (t *testManager) TestViperMultipleConfig() {
	config := &testConfig{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()
	vp.SetConfigType("yml")

	manager := NewManager(config, cmd, vp)
	manager.SetViperConfig("yml", []byte(`
naru:
  a: "10"
  b: "2"
`))
	manager.SetViperConfig("yml", []byte(`
naru:
  a: "3"
`))
	_, err := manager.Merge()
	t.NoError(err)

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(3, merged)
	}

	{
		var merged string
		err := manager.GetValue("b", &merged)
		t.NoError(err)
		t.Equal("2", merged)
	}
}

func (t *testManager) TestEnv() {
	config := &testConfig{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)
	manager.SetEnvLookupFunc(func(s string) (string, bool) {
		switch s {
		case "NARU_A":
			return "33", true
		default:
			return "", false
		}
	})
	_, err := manager.Merge()
	t.NoError(err)

	{
		var merged int
		err := manager.GetValue("a", &merged)
		t.NoError(err)
		t.Equal(33, merged)
	}
}

func (t *testManager) TestEnvs() {
	config := &testConfig{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)

	envs := manager.Envs()
	t.Equal(2, len(envs))
	sort.Strings(envs)
	t.Equal("NARU_A", envs[0])
	t.Equal("NARU_B", envs[1])
}

func (t *testManager) TestFlagSet() {
	config := &testConfig{
		A: 1,
		B: "2",
	}

	cmd := &cobra.Command{
		Use:   "naru",
		Short: "naru",
	}
	cmd.SetOutput(ioutil.Discard)

	vp := viper.New()

	manager := NewManager(config, cmd, vp)

	var flagNames []string
	fs := manager.FlagSet()
	fs.VisitAll(func(p *pflag.Flag) {
		flagNames = append(flagNames, p.Name)
	})

	t.Equal(2, len(flagNames))

	sort.Strings(flagNames)
	t.Equal("a", flagNames[0])
	t.Equal("b", flagNames[1])
}

func TestManager(t *testing.T) {
	suite.Run(t, new(testManager))
}
