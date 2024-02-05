package functions

import "errors"

type Function struct {
	FunctionID  string
	VersionID   string
	Name        string
	Description string
	Version     string
	Uses        int32
	Input       map[string]ParamMetadata
	Output      map[string]ParamMetadata
	Options     Options
	Contracts   string
	CreatedAt   string
	DeletedAt   string
	UpdatedAt   string
	Versions    []Version
}

type Version struct {
	VersionID   string
	FunctionID  string
	Description string
	Version     string
	Input       string
	Output      string
	Options     string
	Contracts   string
	CreatedAt   string
	UpdatedAt   string
	DeletedAt   string
}

type ParamMetadata struct {
	Type        string
	Description string
	Items       *ParamMetadata
	Properties  map[string]ParamMetadata
}

type Options struct {
	Private bool
	Type    string
	Input   map[string]interface{}
	Output  map[string]ParamMetadata
}

const (
	AsyncFlag = "async"
	SyncFlag  = "sync"
)

func (f *Function) IsAsync() (result bool, err error) {
	switch f.Options.Type {
	case AsyncFlag:
		return true, nil
	case SyncFlag:
		return false, nil
	default:
		return false, errors.New("invalid option type. Expected 'sync' or 'async'")
	}
}
