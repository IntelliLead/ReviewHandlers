package dbModel

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "strings"
)

type AttributeAction struct {
    Action enum.Action // "update" or "delete"
    Name   string      // Name of the attribute
    Value  interface{} // Value to set (for updates only)
}

// NewAttributeAction creates a new AttributeAction with data validation and normalization
func NewAttributeAction(action enum.Action, name string, value interface{}) (AttributeAction, error) {
    aa := AttributeAction{
        Action: action,
        Name:   name,
        Value:  value,
    }

    // For safety in case of typo
    // Ensure first letter is lowercase
    name = strings.ToLower(string(name[0])) + name[1:]
    // find position of '.' and ensure next letter is lowercase
    for i := 0; i < len(name); i++ {
        if name[i] == '.' {
            // check if there are 2 letters after '.'
            if i+2 >= len(name) {
                return AttributeAction{}, errors.New(fmt.Sprintf("Attribute name '%s' is invalid", name))
            }
            name = name[:i+1] + strings.ToLower(string(name[i+1])) + name[i+2:]
        }
    }

    err := aa.validate()
    if err != nil {
        return AttributeAction{}, err
    }

    return aa, nil
}

func (a *AttributeAction) validate() error {
    if a.Action == enum.ActionAppend {
        // assert value is slice
        if _, ok := a.Value.([]interface{}); !ok {
            return errors.New(fmt.Sprintf("Value for append action must be a slice, got %T", a.Value))
        }
    }
    return nil
}

func ValidateUniqueAttributeNames(actions []AttributeAction) error {
    if len(actions) == 0 {
        return errors.New(fmt.Sprintf("No actions provided to UpdateAttributes"))
    }

    uniqueNames := make(map[string]bool)
    for _, action := range actions {
        if _, ok := uniqueNames[action.Name]; ok {
            return errors.New(fmt.Sprintf("Duplicate attribute name '%s' in UpdateAttributes", action.Name))
        }
        uniqueNames[action.Name] = true
    }
    return nil
}
