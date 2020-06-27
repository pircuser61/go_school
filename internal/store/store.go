package store

import (
	"errors"
	"reflect"
	"sync"
)

var (
	errUnknownKey      = errors.New("unknown key in context")
	errValueNotAString = errors.New("value not a string")
	errValueNotABool   = errors.New("value not a bool")
	errNoSuchKey       = errors.New("no such key")
)

type VariableStore struct {
	mut    *sync.Mutex
	Values map[string]interface{}
	Steps  []string
	Errors []string
}

func NewStore() *VariableStore {
	s := VariableStore{mut: &sync.Mutex{}, Values: make(map[string]interface{})}
	s.Steps = make([]string, 0)
	s.Errors = make([]string, 0)

	return &s
}

func (c *VariableStore) AddStep(name string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.Steps = append(c.Steps, name)
}

func (c *VariableStore) AddError(name error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.Errors = append(c.Errors, name.Error())
}

func (c *VariableStore) GetValue(name string) (interface{}, bool) {
	c.mut.Lock()
	defer c.mut.Unlock()
	val, ok := c.Values[name]

	return val, ok
}

func (c *VariableStore) GetArray(name string) ([]interface{}, bool) {
	c.mut.Lock()
	defer c.mut.Unlock()
	val, ok := c.Values[name]
	if !ok {
		return nil, ok
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Slice:
		return val.([]interface{}), ok
	}
	return nil, ok
}

func (c *VariableStore) GrabOutput() (interface{}, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	return c.Values, nil
}

func (c *VariableStore) GrabSteps() ([]string, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	return c.Steps, nil
}

func (c *VariableStore) GrabErrors() ([]string, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	return c.Errors, nil
}

func (c *VariableStore) GetString(name string) (string, error) {
	v, ok := c.GetValue(name)
	if !ok {
		return "", errUnknownKey
	}

	s, ok := v.(string)
	if !ok {
		return "", errValueNotAString
	}

	return s, nil
}

func (c *VariableStore) GetBool(name string) (bool, error) {
	v, ok := c.GetValue(name)
	if !ok {
		return false, errUnknownKey
	}

	s, ok := v.(bool)
	if !ok {
		return false, errValueNotABool
	}

	return s, nil
}

func (c *VariableStore) GetStringWithInput(inMap map[string]string, key string) (string, error) {
	inKey, ok := inMap[key]
	if !ok {
		return "", errNoSuchKey
	}

	return c.GetString(inKey)
}

func (c *VariableStore) GetBoolWithInput(inMap map[string]string, key string) (bool, error) {
	inKey, ok := inMap[key]
	if !ok {
		return false, errNoSuchKey
	}

	return c.GetBool(inKey)
}

func (c *VariableStore) SetValue(name string, value interface{}) {
	c.mut.Lock()
	defer c.mut.Unlock()

	switch v := value.(type) {
	case string:
		c.Values[name] = v
	case bool:
		c.Values[name] = v
	case int:
		c.Values[name] = v
	default:
		c.Values[name] = value
	}
}

func (c *VariableStore) SetStringWithOutput(outMap map[string]string, key, val string) error {
	outKey, ok := outMap[key]
	if !ok {
		return errNoSuchKey
	}

	c.SetValue(outKey, val)

	return nil
}

func (c *VariableStore) SetBoolWithOutput(outMap map[string]string, key string, val bool) error {
	outKey, ok := outMap[key]
	if !ok {
		return errNoSuchKey
	}

	c.SetValue(outKey, val)

	return nil
}
