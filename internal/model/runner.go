package model

type Runner interface {
	Run(ctx *Context) error
	Next() string
}
