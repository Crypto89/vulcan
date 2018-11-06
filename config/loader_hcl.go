package config

import (
	"fmt"
	"io/ioutil"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/hcl/ast"

	"github.com/hashicorp/hcl"
)

type hclConfigurable struct {
	File string
	Root *ast.File
}

func (t *hclConfigurable) Config() (*Config, error) {
	validKeys := map[string]struct{}{
		"file":     struct{}{},
		"group":    struct{}{},
		"package":  struct{}{},
		"service":  struct{}{},
		"user":     struct{}{},
		"variable": struct{}{},
	}

	list, ok := t.Root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: file doesn't contain a root object")
	}

	config := new(Config)

	if o := list.Filter("variable"); len(o.Items) > 0 {
		var err error
		config.Variables, err = loadVariablesHcl(o)
		if err != nil {
			return nil, err
		}
	}

	if o := list.Filter("file"); len(o.Items) > 0 {
		var err error
		config.Files, err = loadFilesHcl(o)
		if err != nil {
			return nil, err
		}
	}

	// Check for invalid keys
	for _, item := range list.Items {
		if len(item.Keys) == 0 {
			// Not sure how this would happen, but let's avoid a panic
			continue
		}

		k := item.Keys[0].Token.Value().(string)
		if _, ok := validKeys[k]; ok {
			continue
		}

		config.unknownKeys = append(config.unknownKeys, k)
	}

	return config, nil
}

func loadFileHcl(root string) (configurable, error) {
	// Load the file
	d, err := ioutil.ReadFile(root)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", root, err)
	}

	// Parse
	hclRoot, err := hcl.Parse(string(d))
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %s", root, err)
	}

	result := &hclConfigurable{
		File: root,
		Root: hclRoot,
	}

	return result, nil
}

func loadFilesHcl(list *ast.ObjectList) ([]*File, error) {
	if err := assertAllBlocksHaveNames("file", list); err != nil {
		return nil, err
	}

	list = list.Children()

	result := make([]*File, len(list.Items))
	for _, item := range list.Items {
		unwrapHCLObjectKeysFromJSON(item, 1)

		if len(item.Keys) != 1 {
			return nil, fmt.Errorf(
				"position %s: 'file' must be follow by exactly one string: a name",
				item.Pos(),
			)
		}

		n := item.Keys[0].Token.Value().(string)
		if !NameRegexp.MatchString(n) {
			return nil, fmt.Errorf(
				"position %s: 'file' name must match regular expression: %s",
				item.Pos(), NameRegexp)
		}

		valid := []string{"destination", "content", "user", "group", "mode"}
		if err := checkHCLKeys(item.Val, valid); err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf(
				"variable[%s]:", n))
		}

		var hclFile File
		if err := hcl.DecodeObject(&hclFile, item.Val); err != nil {
			return nil, err
		}

		result = append(result, &hclFile)
	}

	return result, nil
}

func loadVariablesHcl(list *ast.ObjectList) ([]*Variable, error) {
	if err := assertAllBlocksHaveNames("file", list); err != nil {
		return nil, err
	}

	list = list.Children()

	type hclVariable struct {
		DeclaredType string `hcl:"type"`
		Default      interface{}
		Description  string
		Fields       []string `hcl:",decodedFields"`
	}

	result := make([]*Variable, len(list.Items))
	for _, item := range list.Items {
		unwrapHCLObjectKeysFromJSON(item, 1)

		if len(item.Keys) != 1 {
			return nil, fmt.Errorf(
				"position %s: 'variable' must be followed by exactly one string: a name",
				item.Pos())
		}

		n := item.Keys[0].Token.Value().(string)
		if !NameRegexp.MatchString(n) {
			return nil, fmt.Errorf(
				"position %s: 'variable' name must match regular expression: %s",
				item.Pos(), NameRegexp)
		}

		valid := []string{"type", "default", "description"}
		if err := checkHCLKeys(item.Val, valid); err != nil {
			return nil, multierror.Prefix(err, fmt.Sprintf(
				"variable[%s]:", n))
		}

		var hclVar hclVariable
		if err := hcl.DecodeObject(&hclVar, item.Val); err != nil {
			return nil, err
		}

		if ms, ok := hclVar.Default.([]map[string]interface{}); ok {
			def := make(map[string]interface{})
			for _, m := range ms {
				for k, v := range m {
					def[k] = v
				}
			}

			hclVar.Default = def
		}

		newVar := &Variable{
			Name:         n,
			DeclaredType: hclVar.DeclaredType,
			Default:      hclVar.Default,
			Description:  hclVar.Description,
		}
		if err := newVar.ValidateTypeAndDefault(); err != nil {
			return nil, err
		}

		result = append(result, newVar)
	}

	return result, nil
}

func assertAllBlocksHaveNames(name string, list *ast.ObjectList) error {
	if elem := list.Elem(); len(elem.Items) != 0 {
		switch et := elem.Items[0].Val.(type) {
		case *ast.ObjectType:
			pos := et.Lbrace
			return fmt.Errorf("%s: %q must be followed by a name", pos, name)
		default:
			pos := elem.Items[0].Val.Pos()
			return fmt.Errorf("%s: %q must be a configuration block", pos, name)
		}
	}

	return nil
}

func checkHCLKeys(node ast.Node, valid []string) error {
	var list *ast.ObjectList
	switch n := node.(type) {
	case *ast.ObjectList:
		list = n
	case *ast.ObjectType:
		list = n.List
	default:
		return fmt.Errorf("cannot check HCL keys of type %T", n)
	}

	validMap := make(map[string]struct{}, len(valid))
	for _, v := range valid {
		validMap[v] = struct{}{}
	}

	var result error
	for _, item := range list.Items {
		key := item.Keys[0].Token.Value().(string)
		if _, ok := validMap[key]; !ok {
			result = multierror.Append(result, fmt.Errorf(
				"invalid key: %s", key))
		}
	}

	return result
}

func unwrapHCLObjectKeysFromJSON(item *ast.ObjectItem, depth int) {
	if len(item.Keys) > depth && item.Keys[0].Token.JSON {
		for len(item.Keys) > depth {
			// Pop off the last key
			n := len(item.Keys)
			key := item.Keys[n-1]
			item.Keys[n-1] = nil
			item.Keys = item.Keys[:n-1]

			// Wrap our value in a list
			item.Val = &ast.ObjectType{
				List: &ast.ObjectList{
					Items: []*ast.ObjectItem{
						&ast.ObjectItem{
							Keys: []*ast.ObjectKey{key},
							Val:  item.Val,
						},
					},
				},
			}
		}
	}
}
