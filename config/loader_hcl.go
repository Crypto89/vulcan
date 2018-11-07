package config

import (
	"fmt"
	"io/ioutil"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	log "github.com/sirupsen/logrus"
)

type hclConfigurable struct {
	File string
	Root *ast.File
}

func (t *hclConfigurable) Config() (*Config, error) {
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

	{
		var err error
		config.Resources, err = loadResourcesHcl(list)
		if err != nil {
			return nil, err
		}
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

func loadResourcesHcl(list *ast.ObjectList) (map[string][]*Resource, error) {
	list = list.Children()
	result := make(map[string][]*Resource)

	for _, item := range list.Items {
		if len(item.Keys) == 0 {
			// Not sure how this would happen, but let's avoid a panic
			continue
		}

		t := item.Keys[0].Token.Value().(string)
		k := item.Keys[1].Token.Value().(string)

		if t == "variable" {
			// we already handled this, skip
			continue
		}

		if !NameRegexp.MatchString(k) {
			return nil, fmt.Errorf("position %s: '%s' name must match regular expression: %s", item.Pos(), t, NameRegexp)
		}

		var listVal *ast.ObjectList
		if ot, ok := item.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return nil, fmt.Errorf("resource %s[%s]: should be an object", t, k)
		}

		var config map[string]interface{}
		if err := hcl.DecodeObject(&config, item.Val); err != nil {
			return nil, fmt.Errorf("Error reading config for %s[%s]: %s", t, k, err)
		}

		log.Debugf("Found resource: %s[%s]", t, k)
		log.Debugf("Found config: %#v", config)

		delete(config, "depends_on")

		rawConfig, err := NewRawConfig(config)
		if err != nil {
			return nil, fmt.Errorf("Error reading config for %s[%s]: %s", t, k, err)
		}

		var dependsOn []string
		if o := listVal.Filter("depends_on"); len(o.Items) > 0 {
			err := hcl.DecodeObject(&dependsOn, o.Items[0].Val)
			if err != nil {
				return nil, fmt.Errorf("Error reading depends_on for %s[%s]: %s", t, k, err)
			}
		}

		r := &Resource{
			Name:      k,
			Keys:      config,
			RawConfig: rawConfig,
		}

		if _, ok := result[t]; !ok {
			result[t] = []*Resource{}
		}

		result[t] = append(result[t], r)
	}

	// for _, item := range list.Items {
	// 	unwrapHCLObjectKeysFromJSON(item, 1)

	// 	if len(item.Keys) != 1 {
	// 		return nil, fmt.Errorf(
	// 			"position %s: 'file' must be follow by exactly one string: a name",
	// 			item.Pos(),
	// 		)
	// 	}

	// 	valid := []string{"destination", "content", "user", "group", "mode"}
	// 	if err := checkHCLKeys(item.Val, valid); err != nil {
	// 		return nil, multierror.Prefix(err, fmt.Sprintf(
	// 			"variable[%s]:", n))
	// 	}

	// 	var hclFile File
	// 	if err := hcl.DecodeObject(&hclFile, item.Val); err != nil {
	// 		return nil, err
	// 	}

	// 	result = append(result, &hclFile)
	// }

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
