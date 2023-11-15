package mocks

import (
	http "net/http"

	mock "github.com/stretchr/testify/mock"
)

// RoundTripper is an autogenerated mock type for the RoundTripper type
type RoundTripper struct {
	mock.Mock
}

// RoundTrip provides a mock function with given fields: _a0
func (_m *RoundTripper) RoundTrip(_a0 *http.Request) (*http.Response, error) {
	ret := _m.Called(_a0)

	var r0 *http.Response
	if rf, ok := ret.Get(0).(func(*http.Request) *http.Response); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*http.Response)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*http.Request) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewRoundTripper interface {
	mock.TestingT
	Cleanup(func())
}

// NewRoundTripper creates a new instance of RoundTripper. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewRoundTripper(t mockConstructorTestingTNewRoundTripper) *RoundTripper {
	mock := &RoundTripper{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
