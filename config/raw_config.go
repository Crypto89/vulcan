package config

import (
	"sync"

	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/reflectwalk"
)

const UnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

type RawConfig struct {
	Key string
	Raw map[string]interface{}

	Interpoliations []ast.Node
	Variables       map[string]InterpolatedVariable

	lock        sync.Mutex
	config      map[string]interface{}
	unknownKeys []string
}

func NewRawConfig(raw map[string]interface{}) (*RawConfig, error) {
	result := &RawConfig{Raw: raw}
	if err := result.init(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *RawConfig) init() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.config = r.Raw
	r.Interpoliations = nil
	r.Variables = nil

	fn := func(node ast.Node) (interface{}, error) {
		r.Interpoliations = append(r.Interpoliations, node)
		vars, err := DetectVariables(node)
		if err != nil {
			return "", err
		}

		for _, v := range vars {
			if r.Variables == nil {
				r.Variables = make(map[string]InterpolatedVariable)
			}

			r.Variables[v.FullKey()] = v
		}

		return "", nil
	}

	walker := &interpolationWalker{F: fn}
	if err := reflectwalk.Walk(r.Raw, walker); err != nil {
		return err
	}

	return nil
}

func (r *RawConfig) interpolate(fn interpolationWalkerFunc) error {
	config, err := copystructure.Copy(r.Raw)
	if err != nil {
		return err
	}
	r.config = config.(map[string]interface{})

	w := &interpolationWalker{F: fn, Replace: true}
	err = reflectwalk.Walk(r.config, w)
	if err != nil {
		return err
	}

	r.unknownKeys = w.unknownKeys
	return nil
}

func (r *RawConfig) Interpolate(vs map[string]ast.Variable) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	config := langEvalConfig(vs)
	return r.interpolate(func(root ast.Node) (interface{}, error) {
		result, err := hil.Eval(root, config)
		if err != nil {
			return "", err
		}

		return result.Value, nil
	})
}

func langEvalConfig(vs map[string]ast.Variable) *hil.EvalConfig {
	funcMap := make(map[string]ast.Function)
	for k, v := range Funcs() {
		funcMap[k] = v
	}
	funcMap["lookup"] = interpolationFuncLookup(vs)
	funcMap["keys"] = interpolationFuncKeys(vs)
	funcMap["values"] = interpolationFuncValues(vs)

	return &hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap:  vs,
			FuncMap: funcMap,
		},
	}
}
