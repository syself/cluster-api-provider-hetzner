// Code generated by mockery v2.10.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// GetHardwareDetailsCPUArch provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUArch() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsCPUClockGigahertz provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUClockGigahertz() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsCPUCores provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUCores() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsCPUFlags provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUFlags() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsCPUModel provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUModel() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsCPUThreads provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUThreads() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsNics provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsNics() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsRam provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsRam() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHardwareDetailsStorage provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsStorage() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// GetHostName provides a mock function with given fields:
func (_m *Client) GetHostName() sshclient.Output {
	ret := _m.Called()

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}
