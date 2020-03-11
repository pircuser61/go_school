package model

import "sync"

type Context struct {
	mut sync.Mutex
	values map[string]interface{}
}

func NewContext() Context  {
	return Context{mut: sync.Mutex{}, values: make(map[string]interface{})}
}

func (c Context) GetValue(name string) interface{} {
	c.mut.Lock()
	defer c.mut.Unlock()
	if val, ok := c.values[name]; ok {
		return val
	}
	return nil
}

func (c *Context) SetValue(name string, value interface{})  {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.values[name] = value
}
