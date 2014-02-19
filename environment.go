package tachyon

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/vektra/tachyon/lisp"
	"strings"
	"sync"
)

type Environment struct {
	ShowCommandOutput bool

  report Reporter
}

func (e *Environment) Init() {
  e.report = sCLIReporter
}

var cTemplateStart = []byte(`{{`)
var cTemplateEnd = []byte(`}}`)
var cExprStart = []byte(`$(`)
var cExprEnd = []byte(`)`)

var eUnclosedTemplate = errors.New("Unclosed template")
var eUnclosedExpr = errors.New("Unclosed lisp expression")

func (env *Environment) expandTemplates(args string, pe *PlayEnv) (string, error) {
	a := []byte(args)

	var buf bytes.Buffer

	for {
		idx := bytes.Index(a, cTemplateStart)

		if idx == -1 {
			buf.Write(a)
			break
		}

		buf.Write(a[:idx])

		in := a[idx+2:]

		fin := bytes.Index(in, cTemplateEnd)

		if fin == -1 {
			return "", eUnclosedTemplate
		}

		name := bytes.TrimSpace(in[:fin])

		if val, ok := pe.Get(string(name)); ok {
			switch val := val.(type) {
			case int64, int:
				buf.WriteString(fmt.Sprintf("%d", val))
			default:
				buf.WriteString(fmt.Sprintf("%s", val))
			}

			a = in[fin+2:]
		} else {
			return "", fmt.Errorf("Undefined variable: %s", string(name))
		}
	}

	return buf.String(), nil
}

func findExprClose(buf []byte) int {
	opens := 0

	for idx, r := range buf {
		switch r {
		case ')':
			opens--

			if opens == 0 {
				return idx
			}

		case '(':
			opens++
		}
	}

	return -1
}

type SimpleMap map[string]string

func (env *Environment) ParseSimpleMap(args string, pe *PlayEnv) (SimpleMap, error) {
	args, err := env.ExpandVars(args, pe)

	if err != nil {
		return nil, err
	}

	sm := make(SimpleMap)

	parts := strings.Split(args, " ")

	for _, part := range parts {
		ec := strings.SplitN(part, "=", 2)

		if len(ec) == 2 {
			sm[ec[0]] = ec[1]
		} else {
			sm[part] = "true"
		}
	}

	return sm, nil
}

func missingValue(key string) error {
	return fmt.Errorf("Missing value for key '%s'", key)
}

func (env *Environment) ExpandVars(args string, pe *PlayEnv) (string, error) {
	args, err := env.expandTemplates(args, pe)

	if err != nil {
		return "", err
	}

	a := []byte(args)

	var buf bytes.Buffer

	for {
		idx := bytes.Index(a, cExprStart)

		if idx == -1 {
			buf.Write(a)
			break
		}

		buf.Write(a[:idx])

		in := a[idx+1:]

		fin := findExprClose(in)

		if fin == -1 {
			return "", eUnclosedExpr
		}

		sexp := in[:fin+1]

		val, err := lisp.EvalString(string(sexp), pe.lispScope)

		if err != nil {
			return "", err
		}

		// fmt.Printf("%s => %s\n", string(sexp), val.Inspect())

		buf.WriteString(val.String())
		a = in[fin+1:]
	}

	return buf.String(), nil
}

type PlayEnv struct {
	Vars      Vars
	lispScope *lisp.Scope
	to_notify map[string]struct{}
	async     chan *AsyncAction
	wait      sync.WaitGroup
}

func (pe *PlayEnv) Init() {
	pe.to_notify = make(map[string]struct{})
	pe.lispScope.AddEnv()
	pe.async = make(chan *AsyncAction)

	go pe.handleAsync()
}

func (pe *PlayEnv) Set(key string, val interface{}) {
	pe.Vars[key] = val

	switch lv := val.(type) {
	case int64:
		pe.lispScope.Set(key, lisp.NumberValue(lv))
	default:
		pe.lispScope.Set(key, lisp.StringValue(fmt.Sprintf("%s", lv)))
	}
}

func (pe *PlayEnv) Get(key string) (interface{}, bool) {
	v, ok := pe.Vars[key]

	return v, ok
}

func (pe *PlayEnv) AddNotify(n string) {
	pe.to_notify[n] = struct{}{}
}

func (pe *PlayEnv) ShouldRunHandler(name string) bool {
	_, ok := pe.to_notify[name]

	return ok
}

func (pe *PlayEnv) AsyncChannel() chan *AsyncAction {
	return pe.async
}
