package cvc

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type testSuiteNormalizeVar struct {
	suite.Suite
}

func (t *testSuiteNormalizeVar) TestSimple() {
	s := "FindMe"
	e := "find_me"
	t.Equal(e, NormalizeVar(s, ""))
}

func (t *testSuiteNormalizeVar) Test_ABCde() {
	s := "ABCDe"
	e := "abcde"
	t.Equal(e, NormalizeVar(s, ""))
}

func (t *testSuiteNormalizeVar) Test_abcde() {
	s := "abcde"
	e := "abcde"
	t.Equal(e, NormalizeVar(s, ""))
}

func (t *testSuiteNormalizeVar) Test_JSONRpc() {
	s := "JSONRpc"
	e := "jsonrpc"
	t.Equal(e, NormalizeVar(s, ""))
}

func (t *testSuiteNormalizeVar) TestDot_SimpleJSONRpc() {
	s := "Simple.JSONRpc"
	e := "simple.jsonrpc"
	t.Equal(e, NormalizeVar(s, ""))
}

func (t *testSuiteNormalizeVar) TestDot_ABCDeJSONRpc() {
	s := "ABCDe.JSONRpc"
	e := "abcde.jsonrpc"
	t.Equal(e, NormalizeVar(s, "."))
}

func (t *testSuiteNormalizeVar) TestDot_AllBind() {
	s := "All.Bind"
	e := "all.bind"
	t.Equal(e, NormalizeVar(s, "."))
}

func (t *testSuiteNormalizeVar) TestCapital() {
	s := "AllBindK"
	e := "all_bind_k"
	t.Equal(e, NormalizeVar(s, ""))
}

func TestNormalizeVar(t *testing.T) {
	suite.Run(t, new(testSuiteNormalizeVar))
}
