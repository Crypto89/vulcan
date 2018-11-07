package config

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hil/ast"
)

type InterpolatedVariable interface {
	FullKey() string
}

type UserVariable struct {
	Name string
	Elem string

	key string
}

func NewInterpolatedVariable(v string) (InterpolatedVariable, error) {
	if strings.HasPrefix(v, "var.") {
		return NewUserVariable(v)
	}

	return nil, fmt.Errorf("not yet implemented")
}

func NewUserVariable(key string) (*UserVariable, error) {
	name := key[len("var."):]
	elem := ""
	if idx := strings.Index(name, "."); idx > -1 {
		elem = name[idx+1:]
		name = name[:idx]
	}

	if len(elem) > 0 {
		return nil, fmt.Errorf("invalid dot index found: 'var.%s.%s'. Valies in maps and lists can be referenced using square bracket indexing, like: 'var.mymap[\"key\"]' or 'var.mylist[1]'", name, elem)
	}

	return &UserVariable{
		key:  key,
		Name: name,
		Elem: elem,
	}, nil
}

func (v *UserVariable) FullKey() string {
	return v.key
}

func DetectVariables(root ast.Node) ([]InterpolatedVariable, error) {
	var result []InterpolatedVariable
	var resultErr error

	fn := func(n ast.Node) ast.Node {
		if resultErr != nil {
			return n
		}

		switch vn := n.(type) {
		case *ast.VariableAccess:
			v, err := NewInterpolatedVariable(vn.Name)
			if err != nil {
				resultErr = err
				return n
			}
			result = append(result, v)
		case *ast.Index:
			if va, ok := vn.Target.(*ast.VariableAccess); ok {
				v, err := NewInterpolatedVariable(va.Name)
				if err != nil {
					resultErr = err
					return n
				}
				result = append(result, v)
			}
		default:
			return n
		}

		return n
	}

	root.Accept(fn)

	if resultErr != nil {
		return nil, resultErr
	}

	return result, nil
}
