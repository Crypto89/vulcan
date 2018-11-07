package config

import (
	"fmt"
	"math"
	"sort"

	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
)

// stringSliceToVariableValue converts a string slice into the value
// required to be returned from interpolation functions which return
// TypeList.
func stringSliceToVariableValue(values []string) []ast.Variable {
	output := make([]ast.Variable, len(values))
	for index, value := range values {
		output[index] = ast.Variable{
			Type:  ast.TypeString,
			Value: value,
		}
	}
	return output
}

func Funcs() map[string]ast.Function {
	// TODO add funcs from terraform config/interpolate_funcs.go
	return map[string]ast.Function{
		"abs": interpolationFuncAbs(),
	}
}

// interpolationFuncAbs returns the absolute value of a given float.
func interpolationFuncAbs() ast.Function {
	return ast.Function{
		ArgTypes:   []ast.Type{ast.TypeFloat},
		ReturnType: ast.TypeFloat,
		Callback: func(args []interface{}) (interface{}, error) {
			return math.Abs(args[0].(float64)), nil
		},
	}
}

func interpolationFuncLookup(vs map[string]ast.Variable) ast.Function {
	return ast.Function{
		ArgTypes:     []ast.Type{ast.TypeMap, ast.TypeString},
		ReturnType:   ast.TypeString,
		Variadic:     true,
		VariadicType: ast.TypeString,
		Callback: func(args []interface{}) (interface{}, error) {
			defaultValue := ""
			defaultValueSet := false
			if len(args) > 2 {
				defaultValue = args[2].(string)
				defaultValueSet = true
			}
			if len(args) > 3 {
				return "", fmt.Errorf("lookup() take no more than three arguments")
			}

			index := args[1].(string)
			mapVar := args[0].(map[string]ast.Variable)

			v, ok := mapVar[index]
			if !ok {
				if defaultValueSet {
					return defaultValue, nil
				} else {
					return "", fmt.Errorf("lookup failed to find '%s'", index)
				}
			}
			if v.Type != ast.TypeString {
				return nil, fmt.Errorf("lookup() may only be used with flat maps, this map contains elements of %s", v.Type.Printable())
			}

			return v.Value.(string), nil
		},
	}
}

func interpolationFuncKeys(vs map[string]ast.Variable) ast.Function {
	return ast.Function{
		ArgTypes:   []ast.Type{ast.TypeMap},
		ReturnType: ast.TypeList,
		Callback: func(args []interface{}) (interface{}, error) {
			mapVar := args[0].(map[string]ast.Variable)
			keys := make([]string, 0)

			for k, _ := range mapVar {
				keys = append(keys, k)
			}

			sort.Strings(keys)

			// Keys are guaranteed to be strings
			return stringSliceToVariableValue(keys), nil
		},
	}
}

func interpolationFuncValues(vs map[string]ast.Variable) ast.Function {
	return ast.Function{
		ArgTypes:   []ast.Type{ast.TypeMap},
		ReturnType: ast.TypeList,
		Callback: func(args []interface{}) (interface{}, error) {
			mapVar := args[0].(map[string]ast.Variable)
			keys := make([]string, 0)

			for k, _ := range mapVar {
				keys = append(keys, k)
			}

			sort.Strings(keys)

			values := make([]string, len(keys))
			for index, key := range keys {
				if value, ok := mapVar[key].Value.(string); ok {
					values[index] = value
				} else {
					return "", fmt.Errorf("values(): %q has element with bad type %s",
						key, mapVar[key].Type)
				}
			}

			variable, err := hil.InterfaceToVariable(values)
			if err != nil {
				return nil, err
			}

			return variable.Value, nil
		},
	}
}
