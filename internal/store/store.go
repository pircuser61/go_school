package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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

//nolint:gocritic // fix later
type VariableStore struct {
	// TODO: RWMutex?
	sync.Mutex
	State      map[string]json.RawMessage
	Values     map[string]interface{}
	Steps      []string
	Errors     []string
	StopPoints StopPoints `json:"-"`
}

func NewStore() *VariableStore {
	s := VariableStore{
		State:      make(map[string]json.RawMessage),
		Values:     make(map[string]interface{}),
		Steps:      make([]string, 0),
		Errors:     make([]string, 0),
		StopPoints: StopPoints{},
	}

	return &s
}

type VariableExecutor struct {
	People        []string `json:"people"`
	GroupID       string   `json:"group_id"`
	GroupName     string   `json:"group_name"`
	GroupLimit    int      `json:"group_limit"`
	InitialPeople []string `json:"initial_people"`
}

func NewExecutor() *VariableExecutor {
	e := VariableExecutor{
		People:        make([]string, 0),
		GroupID:       "",
		GroupName:     "",
		GroupLimit:    0,
		InitialPeople: make([]string, 0),
	}

	return &e
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

func (c *VariableStore) Copy() *VariableStore {
	newState := make(map[string]json.RawMessage)
	for k, v := range c.State {
		newState[k] = v
	}

	newValues := make(map[string]interface{})
	for k, v := range c.Values {
		newValues[k] = v
	}

	newSteps := make([]string, len(c.Steps))
	copy(newSteps, c.Steps)

	newErrors := make([]string, len(c.Errors))
	copy(newErrors, c.Errors)

	return &VariableStore{
		Mutex:      sync.Mutex{},
		State:      newState,
		Values:     newValues,
		Steps:      newSteps,
		Errors:     newErrors,
		StopPoints: c.StopPoints,
	}
}

func (c *VariableStore) AddStep(name string) {
	lenSteps := len(c.Steps)

	if lenSteps > 0 && c.Steps[lenSteps-1] == name {
		return
	}

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

// we need to convert structs to maps, structs can be in arrays or other structs
func converter(data interface{}) interface{} {
	bytes, err := json.Marshal(data)
	if err != nil {
		return data
	}

	var newData interface{}

	if unmErr := json.Unmarshal(bytes, &newData); unmErr != nil {
		return data
	}

	return newData
}

func (c *VariableStore) SetValue(name string, value interface{}) {
	c.Lock()
	defer c.Unlock()

	if name == "" {
		return
	}

	for reflect.TypeOf(value).Kind() == reflect.Pointer {
		if reflect.ValueOf(value).IsNil() {
			value = reflect.New(reflect.TypeOf(value).Elem()).Elem().Interface()

			break
		}

		value = reflect.ValueOf(value).Elem().Interface()
	}

	switch v := value.(type) {
	case string:
		c.Values[name] = v
	case bool:
		c.Values[name] = v
	case int:
		c.Values[name] = v
	default:
		c.Values[name] = converter(value)
	}
}

// ClearValues deletes all block's values.
func (c *VariableStore) ClearValues(blockName string) {
	c.Lock()
	defer c.Unlock()

	prefix := blockName + "."

	for k := range c.Values {
		if strings.HasPrefix(k, prefix) {
			delete(c.Values, k)
		}
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

func (c *VariableStore) ReplaceState(stepName string, value json.RawMessage) {
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
