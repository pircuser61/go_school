package store

import (
	"errors"
	"reflect"
	"sync"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

var (
	errUnknownKey      = errors.New("unknown key in context")
	errValueNotAString = errors.New("value not a string")
	errValueNotABool   = errors.New("value not a bool")
	errNoSuchKey       = errors.New("no such key")
	errUnknown         = errors.New("unknown")
)

type VariableStore struct {
	// TODO: RWMutex?
	sync.Mutex
	State      map[string]interface{}
	Values     map[string]interface{}
	Steps      []string
	Errors     []string
	StopPoints StopPoints `json:"-"`
}

func NewStore() *VariableStore {
	s := VariableStore{
		State:      make(map[string]interface{}),
		Values:     make(map[string]interface{}),
		Steps:      make([]string, 0),
		Errors:     make([]string, 0),
		StopPoints: StopPoints{},
	}

	return &s
}

func NewFromStep(step *entity.Step) *VariableStore {
	sp := NewStopPoints(step.Name)
	sp.SetBreakPoints(step.BreakPoints...)

	vs := VariableStore{
		State:      step.State,
		Values:     step.Storage,
		Steps:      step.Steps,
		Errors:     step.Errors,
		StopPoints: *sp,
	}

	return &vs
}

func (c *VariableStore) AddStep(name string) {
	c.Lock()
	defer c.Unlock()

	c.Steps = append(c.Steps, name)
}

func (c *VariableStore) AddError(err error) {
	c.Lock()
	defer c.Unlock()

	if err == nil {
		err = errUnknown
	}

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

func (c *VariableStore) SetStopPoints(points StopPoints) {
	c.Lock()
	defer c.Unlock()

	c.StopPoints = points
}

func (c *VariableStore) GetState(stepName string) (interface{}, bool) {
	c.Lock()
	defer c.Unlock()

	val, ok := c.State[stepName]
	return val, ok
}

func (c *VariableStore) ReplaceState(stepName string, value interface{}) {
	c.Lock()
	defer c.Unlock()

	c.State[stepName] = value
}

type StopPoints struct {
	BreakPoints    map[string]struct{}
	StepOverPoints map[string]struct{}
	ExcludedPoints map[string]struct{}
	StartPoint     string
}

func NewStopPoints(startPoint string) *StopPoints {
	sp := StopPoints{
		BreakPoints:    make(map[string]struct{}),
		StepOverPoints: make(map[string]struct{}),
		ExcludedPoints: make(map[string]struct{}),
		StartPoint:     startPoint,
	}

	return &sp
}

func (sp *StopPoints) SetStepOvers(steps ...string) {
	for _, step := range steps {
		if step != "" {
			sp.StepOverPoints[step] = struct{}{}
		}
	}
}

func (sp *StopPoints) SetBreakPoints(steps ...string) {
	for _, step := range steps {
		if step != "" {
			sp.BreakPoints[step] = struct{}{}
		}
	}
}

func (sp *StopPoints) BreakPointsList() []string {
	breakPoints := make([]string, 0)
	for k := range sp.BreakPoints {
		breakPoints = append(breakPoints, k)
	}

	return breakPoints
}

func (sp *StopPoints) SetExcludedPoints(steps ...string) {
	for _, step := range steps {
		if step != "" {
			sp.ExcludedPoints[step] = struct{}{}
		}
	}
}

func (sp *StopPoints) IsStopPoint(stepName string) bool {
	if _, ok := sp.ExcludedPoints[stepName]; ok {
		return false
	}

	if _, ok := sp.StepOverPoints[stepName]; ok {
		return true
	}

	if _, ok := sp.BreakPoints[stepName]; ok {
		return true
	}

	return false
}
