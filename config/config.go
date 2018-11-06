package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform/helper/hilmapstructure"
)

// NameRegexp is the regular expression that all names (modules, providers,
// resources, etc.) must follow.
var NameRegexp = regexp.MustCompile(`(?i)\A[A-Z0-9_][A-Z0-9\-\_]*\z`)

type configurable interface {
	Config() (*Config, error)
}

type Config struct {
	Dir       string
	Files     []*File
	Variables []*Variable

	unknownKeys []string
}

type File struct {
	Destination string
	Content     string
	User        string
	Group       string
	Mode        string
}

type Variable struct {
	Name         string
	DeclaredType string `mapstructure:"type"`
	Default      interface{}
	Description  string
}

// VariableType is the type of value a variable is holding, and returned
// by the Type() function on variables.
type VariableType byte

const (
	VariableTypeUnknown VariableType = iota
	VariableTypeString
	VariableTypeList
	VariableTypeMap
)

func (v VariableType) Printable() string {
	switch v {
	case VariableTypeString:
		return "string"
	case VariableTypeMap:
		return "map"
	case VariableTypeList:
		return "list"
	default:
		return "unknown"
	}
}

var typeStringMap = map[string]VariableType{
	"string": VariableTypeString,
	"map":    VariableTypeMap,
	"list":   VariableTypeList,
}

// Type returns the type of variable this is.
func (v *Variable) Type() VariableType {
	if v.DeclaredType != "" {
		declaredType, ok := typeStringMap[v.DeclaredType]
		if !ok {
			return VariableTypeUnknown
		}

		return declaredType
	}

	return v.inferTypeFromDefault()
}

// ValidateTypeAndDefault ensures that default variable value is compatible
// with the declared type (if one exists), and that the type is one which is
// known to Terraform
func (v *Variable) ValidateTypeAndDefault() error {
	// If an explicit type is declared, ensure it is valid
	if v.DeclaredType != "" {
		if _, ok := typeStringMap[v.DeclaredType]; !ok {
			validTypes := []string{}
			for k := range typeStringMap {
				validTypes = append(validTypes, k)
			}
			return fmt.Errorf(
				"Variable '%s' type must be one of [%s] - '%s' is not a valid type",
				v.Name,
				strings.Join(validTypes, ", "),
				v.DeclaredType,
			)
		}
	}

	if v.DeclaredType == "" || v.Default == nil {
		return nil
	}

	if v.inferTypeFromDefault() != v.Type() {
		return fmt.Errorf("'%s' has a default value which is not of type '%s' (got '%s')",
			v.Name, v.DeclaredType, v.inferTypeFromDefault().Printable())
	}

	return nil
}

// inferTypeFromDefault contains the logic for the old method of inferring
// variable types - we can also use this for validating that the declared
// type matches the type of the default value
func (v *Variable) inferTypeFromDefault() VariableType {
	if v.Default == nil {
		return VariableTypeString
	}

	var s string
	if err := hilmapstructure.WeakDecode(v.Default, &s); err == nil {
		v.Default = s
		return VariableTypeString
	}

	var m map[string]interface{}
	if err := hilmapstructure.WeakDecode(v.Default, &m); err == nil {
		v.Default = m
		return VariableTypeMap
	}

	var l []interface{}
	if err := hilmapstructure.WeakDecode(v.Default, &l); err == nil {
		v.Default = l
		return VariableTypeList
	}

	return VariableTypeUnknown
}
