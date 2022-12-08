package functions

import "errors"

type Function struct {
	FunctionId  string
	VersionId   string
	Name        string
	Description string
	Version     string
	Uses        int32
	Input       map[string]interface{}
	Output      map[string]interface{}
	Options     map[string]interface{}
	CreatedAt   string
	DeletedAt   string
	UpdatedAt   string
	Versions    []Version
}

type Version struct {
	VersionId   string
	FunctionId  string
	Description string
	Version     string
	Input       map[string]interface{}
	Output      map[string]interface{}
	Options     map[string]interface{}
	CreatedAt   string
	UpdatedAt   string
	DeletedAt   string
}

type FieldMetadata struct {
	Type        string
	Description string
}

func (f *Function) GetOptionAsBool(key string) (result bool, err error) {
	if val, ok := f.Options[key]; ok {
		if boolVal, boolOk := val.(bool); boolOk {
			return boolVal, nil
		}
	}
	return false, errors.New("key not found")
}
