package model

type StringEqual struct {
	Operands []string
	Result   bool
	OnTrue   string
	OnFalse  string
}

func (se *StringEqual) Run(ctx *Context) error {
	r := false
	for i := 0; i < len(se.Operands)-1; i++ {
		r = se.Operands[i] == se.Operands[i+1]
		if !r {
			se.Result = false
			return nil
		}
	}
	se.Result = r
	return nil
}

func (se *StringEqual) Next() string {
	if se.Result {
		return se.OnTrue
	}
	return se.OnFalse
}
