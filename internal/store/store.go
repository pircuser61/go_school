package store

import (
	"errors"
	"reflect"
	"sync"

	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
)

var (
	errUnknownKey      = errors.New("unknown key in context")
	errValueNotAString = errors.New("value not a string")
	errValueNotABool   = errors.New("value not a bool")
	errNoSuchKey       = errors.New("no such key")
)

type VariableStore struct {
	sync.Mutex
	Values      map[string]interface{}
	Steps       []string
	Errors      []string
	BreakPoints map[string]struct{}
}

func NewStore() *VariableStore {
	s := VariableStore{
		Values:      make(map[string]interface{}),
		Steps:       make([]string, 0),
		Errors:      make([]string, 0),
		BreakPoints: make(map[string]struct{}),
	}

	return &s
}

func NewFromStep(step *entity.Step, breakPoints map[string]struct{}) *VariableStore {
	return &VariableStore{
		Values:      step.Storage,
		Steps:       step.Steps,
		Errors:      step.Errors,
		BreakPoints: breakPoints,
	}
}

func (c *VariableStore) AddStep(name string) {
	c.Lock()
	defer c.Unlock()

	c.Steps = append(c.Steps, name)
}

func (c *VariableStore) AddError(err error) {
	c.Lock()
	defer c.Unlock()

	c.Errors = append(c.Errors, err.Error())
}

func (c *VariableStore) GetValue(name string) (interface{}, bool) {
	c.Lock()
	defer c.Unlock()
	val, ok := c.Values[name]

	return val, ok
}

func (c *VariableStore) GetArray(name string) ([]interface{}, bool) {
	c.Lock()
	defer c.Unlock()

	val, ok := c.Values[name]
	if !ok {
		return nil, ok
	}

	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Slice {
		return val.([]interface{}), ok
	}

	return nil, ok
}

func (c *VariableStore) GrabStorage() (map[string]interface{}, error) {
	c.Lock()
	defer c.Unlock()

	return c.Values, nil
}

func (c *VariableStore) GrabSteps() ([]string, error) {
	c.Lock()
	defer c.Unlock()

	return c.Steps, nil
}

func (c *VariableStore) GrabErrors() ([]string, error) {
	c.Lock()
	defer c.Unlock()

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
	c.Lock()
	defer c.Unlock()

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

func (c *VariableStore) SetBreakPoints(points map[string]struct{}) {
	c.Lock()
	defer c.Unlock()

	c.BreakPoints = points
}

func (c *VariableStore) IsBreakPointExists(key string) bool {
	c.Lock()
	defer c.Unlock()

	_, ok := c.BreakPoints[key]

	return ok
}
