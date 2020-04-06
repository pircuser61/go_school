package model

import (
	"errors"
	"sync"
)

type VariableStore struct {
	mut    sync.Mutex
	Values map[string]interface{}
}

func NewContext() VariableStore {
	return VariableStore{mut: sync.Mutex{}, Values: make(map[string]interface{})}
}

func (c VariableStore) GetValue(name string) (interface{}, error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	val, ok := c.Values[name]
	if !ok {
		return nil, errors.New("unknown key in context")
	}
	return val, nil
}

func (c VariableStore) GetString(name string) (string, error) {
	v, err := c.GetValue(name)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("value not a string")
	}

	return s, nil
}

func (c VariableStore) GetBool(name string) (bool, error) {
	v, err := c.GetValue(name)
	if err != nil {
		return false, err
	}
	s, ok := v.(bool)
	if !ok {
		return false, errors.New("value not a bool")
	}

	return s, nil
}

func (c VariableStore) GetStringWithInput(inMap map[string]string, key string) (string, error) {
	inKey, ok := inMap[key]
	if !ok {
		return "", errors.New("no such key")
	}
	return c.GetString(inKey)
}

func (c VariableStore) GetBoolWithInput(inMap map[string]string, key string) (bool, error) {
	inKey, ok := inMap[key]
	if !ok {
		return false, errors.New("no such key")
	}
	return c.GetBool(inKey)
}

func (c *VariableStore) SetValue(name string, value interface{}) {
	c.mut.Lock()
	defer c.mut.Unlock()
	switch value.(type) {
	case string:
		v := value.(string)
		c.Values[name] = v
	case bool:
		v := value.(bool)
		c.Values[name] = v
	case int:
		v := value.(bool)
		c.Values[name] = v
	default:
		c.Values[name] = value
	}
}

func (c *VariableStore) SetStringWithOutput(outMap map[string]string, key string, val string) error {
	outKey, ok := outMap[key]
	if !ok {
		return errors.New("no such key")
	}
	c.SetValue(outKey, val)
	return nil
}

func (c *VariableStore) SetBoolWithOutput(outMap map[string]string, key string, val bool) error {
	outKey, ok := outMap[key]
	if !ok {
		return errors.New("no such key")
	}
	c.SetValue(outKey, val)
	return nil
}
