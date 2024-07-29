// Code generated by mockery v2.40.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

type Client_Expecter struct {
	mock *mock.Mock
}

func (_m *Client) EXPECT() *Client_Expecter {
	return &Client_Expecter{mock: &_m.Mock}
}

// CheckCloudInitLogsForSigTerm provides a mock function with given fields:
func (_m *Client) CheckCloudInitLogsForSigTerm() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CheckCloudInitLogsForSigTerm")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CheckCloudInitLogsForSigTerm_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CheckCloudInitLogsForSigTerm'
type Client_CheckCloudInitLogsForSigTerm_Call struct {
	*mock.Call
}

// CheckCloudInitLogsForSigTerm is a helper method to define mock.On call
func (_e *Client_Expecter) CheckCloudInitLogsForSigTerm() *Client_CheckCloudInitLogsForSigTerm_Call {
	return &Client_CheckCloudInitLogsForSigTerm_Call{Call: _e.mock.On("CheckCloudInitLogsForSigTerm")}
}

func (_c *Client_CheckCloudInitLogsForSigTerm_Call) Run(run func()) *Client_CheckCloudInitLogsForSigTerm_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_CheckCloudInitLogsForSigTerm_Call) Return(_a0 sshclient.Output) *Client_CheckCloudInitLogsForSigTerm_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CheckCloudInitLogsForSigTerm_Call) RunAndReturn(run func() sshclient.Output) *Client_CheckCloudInitLogsForSigTerm_Call {
	_c.Call.Return(run)
	return _c
}

// CleanCloudInitInstances provides a mock function with given fields:
func (_m *Client) CleanCloudInitInstances() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CleanCloudInitInstances")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CleanCloudInitInstances_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CleanCloudInitInstances'
type Client_CleanCloudInitInstances_Call struct {
	*mock.Call
}

// CleanCloudInitInstances is a helper method to define mock.On call
func (_e *Client_Expecter) CleanCloudInitInstances() *Client_CleanCloudInitInstances_Call {
	return &Client_CleanCloudInitInstances_Call{Call: _e.mock.On("CleanCloudInitInstances")}
}

func (_c *Client_CleanCloudInitInstances_Call) Run(run func()) *Client_CleanCloudInitInstances_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_CleanCloudInitInstances_Call) Return(_a0 sshclient.Output) *Client_CleanCloudInitInstances_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CleanCloudInitInstances_Call) RunAndReturn(run func() sshclient.Output) *Client_CleanCloudInitInstances_Call {
	_c.Call.Return(run)
	return _c
}

// CleanCloudInitLogs provides a mock function with given fields:
func (_m *Client) CleanCloudInitLogs() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CleanCloudInitLogs")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CleanCloudInitLogs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CleanCloudInitLogs'
type Client_CleanCloudInitLogs_Call struct {
	*mock.Call
}

// CleanCloudInitLogs is a helper method to define mock.On call
func (_e *Client_Expecter) CleanCloudInitLogs() *Client_CleanCloudInitLogs_Call {
	return &Client_CleanCloudInitLogs_Call{Call: _e.mock.On("CleanCloudInitLogs")}
}

func (_c *Client_CleanCloudInitLogs_Call) Run(run func()) *Client_CleanCloudInitLogs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_CleanCloudInitLogs_Call) Return(_a0 sshclient.Output) *Client_CleanCloudInitLogs_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CleanCloudInitLogs_Call) RunAndReturn(run func() sshclient.Output) *Client_CleanCloudInitLogs_Call {
	_c.Call.Return(run)
	return _c
}

// CloudInitStatus provides a mock function with given fields:
func (_m *Client) CloudInitStatus() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CloudInitStatus")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CloudInitStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CloudInitStatus'
type Client_CloudInitStatus_Call struct {
	*mock.Call
}

// CloudInitStatus is a helper method to define mock.On call
func (_e *Client_Expecter) CloudInitStatus() *Client_CloudInitStatus_Call {
	return &Client_CloudInitStatus_Call{Call: _e.mock.On("CloudInitStatus")}
}

func (_c *Client_CloudInitStatus_Call) Run(run func()) *Client_CloudInitStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_CloudInitStatus_Call) Return(_a0 sshclient.Output) *Client_CloudInitStatus_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CloudInitStatus_Call) RunAndReturn(run func() sshclient.Output) *Client_CloudInitStatus_Call {
	_c.Call.Return(run)
	return _c
}

// CreateAutoSetup provides a mock function with given fields: data
func (_m *Client) CreateAutoSetup(data string) sshclient.Output {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for CreateAutoSetup")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(string) sshclient.Output); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CreateAutoSetup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAutoSetup'
type Client_CreateAutoSetup_Call struct {
	*mock.Call
}

// CreateAutoSetup is a helper method to define mock.On call
//   - data string
func (_e *Client_Expecter) CreateAutoSetup(data interface{}) *Client_CreateAutoSetup_Call {
	return &Client_CreateAutoSetup_Call{Call: _e.mock.On("CreateAutoSetup", data)}
}

func (_c *Client_CreateAutoSetup_Call) Run(run func(data string)) *Client_CreateAutoSetup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Client_CreateAutoSetup_Call) Return(_a0 sshclient.Output) *Client_CreateAutoSetup_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CreateAutoSetup_Call) RunAndReturn(run func(string) sshclient.Output) *Client_CreateAutoSetup_Call {
	_c.Call.Return(run)
	return _c
}

// CreateMetaData provides a mock function with given fields: hostName
func (_m *Client) CreateMetaData(hostName string) sshclient.Output {
	ret := _m.Called(hostName)

	if len(ret) == 0 {
		panic("no return value specified for CreateMetaData")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(string) sshclient.Output); ok {
		r0 = rf(hostName)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CreateMetaData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateMetaData'
type Client_CreateMetaData_Call struct {
	*mock.Call
}

// CreateMetaData is a helper method to define mock.On call
//   - hostName string
func (_e *Client_Expecter) CreateMetaData(hostName interface{}) *Client_CreateMetaData_Call {
	return &Client_CreateMetaData_Call{Call: _e.mock.On("CreateMetaData", hostName)}
}

func (_c *Client_CreateMetaData_Call) Run(run func(hostName string)) *Client_CreateMetaData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Client_CreateMetaData_Call) Return(_a0 sshclient.Output) *Client_CreateMetaData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CreateMetaData_Call) RunAndReturn(run func(string) sshclient.Output) *Client_CreateMetaData_Call {
	_c.Call.Return(run)
	return _c
}

// Client_CreateNoCloudDirectory_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateNoCloudDirectory'
type Client_CreateNoCloudDirectory_Call struct {
	*mock.Call
}

// CreateNoCloudDirectory is a helper method to define mock.On call
func (_e *Client_Expecter) CreateNoCloudDirectory() *Client_CreateNoCloudDirectory_Call {
	return &Client_CreateNoCloudDirectory_Call{Call: _e.mock.On("CreateNoCloudDirectory")}
}

func (_c *Client_CreateNoCloudDirectory_Call) Run(run func()) *Client_CreateNoCloudDirectory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_CreateNoCloudDirectory_Call) Return(_a0 sshclient.Output) *Client_CreateNoCloudDirectory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CreateNoCloudDirectory_Call) RunAndReturn(run func() sshclient.Output) *Client_CreateNoCloudDirectory_Call {
	_c.Call.Return(run)
	return _c
}

// CreatePostInstallScript provides a mock function with given fields: data
func (_m *Client) CreatePostInstallScript(data string) sshclient.Output {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for CreatePostInstallScript")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(string) sshclient.Output); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CreatePostInstallScript_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreatePostInstallScript'
type Client_CreatePostInstallScript_Call struct {
	*mock.Call
}

// CreatePostInstallScript is a helper method to define mock.On call
//   - data string
func (_e *Client_Expecter) CreatePostInstallScript(data interface{}) *Client_CreatePostInstallScript_Call {
	return &Client_CreatePostInstallScript_Call{Call: _e.mock.On("CreatePostInstallScript", data)}
}

func (_c *Client_CreatePostInstallScript_Call) Run(run func(data string)) *Client_CreatePostInstallScript_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Client_CreatePostInstallScript_Call) Return(_a0 sshclient.Output) *Client_CreatePostInstallScript_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CreatePostInstallScript_Call) RunAndReturn(run func(string) sshclient.Output) *Client_CreatePostInstallScript_Call {
	_c.Call.Return(run)
	return _c
}

// CreateUserData provides a mock function with given fields: userData
func (_m *Client) CreateUserData(userData string) sshclient.Output {
	ret := _m.Called(userData)

	if len(ret) == 0 {
		panic("no return value specified for CreateUserData")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(string) sshclient.Output); ok {
		r0 = rf(userData)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_CreateUserData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateUserData'
type Client_CreateUserData_Call struct {
	*mock.Call
}

// CreateUserData is a helper method to define mock.On call
//   - userData string
func (_e *Client_Expecter) CreateUserData(userData interface{}) *Client_CreateUserData_Call {
	return &Client_CreateUserData_Call{Call: _e.mock.On("CreateUserData", userData)}
}

func (_c *Client_CreateUserData_Call) Run(run func(userData string)) *Client_CreateUserData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Client_CreateUserData_Call) Return(_a0 sshclient.Output) *Client_CreateUserData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_CreateUserData_Call) RunAndReturn(run func(string) sshclient.Output) *Client_CreateUserData_Call {
	_c.Call.Return(run)
	return _c
}

// DetectLinuxOnAnotherDisk provides a mock function with given fields: sliceOfWwns
func (_m *Client) DetectLinuxOnAnotherDisk(sliceOfWwns []string) sshclient.Output {
	ret := _m.Called(sliceOfWwns)

	if len(ret) == 0 {
		panic("no return value specified for DetectLinuxOnAnotherDisk")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func([]string) sshclient.Output); ok {
		r0 = rf(sliceOfWwns)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_DetectLinuxOnAnotherDisk_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DetectLinuxOnAnotherDisk'
type Client_DetectLinuxOnAnotherDisk_Call struct {
	*mock.Call
}

// DetectLinuxOnAnotherDisk is a helper method to define mock.On call
//   - sliceOfWwns []string
func (_e *Client_Expecter) DetectLinuxOnAnotherDisk(sliceOfWwns interface{}) *Client_DetectLinuxOnAnotherDisk_Call {
	return &Client_DetectLinuxOnAnotherDisk_Call{Call: _e.mock.On("DetectLinuxOnAnotherDisk", sliceOfWwns)}
}

func (_c *Client_DetectLinuxOnAnotherDisk_Call) Run(run func(sliceOfWwns []string)) *Client_DetectLinuxOnAnotherDisk_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]string))
	})
	return _c
}

func (_c *Client_DetectLinuxOnAnotherDisk_Call) Return(_a0 sshclient.Output) *Client_DetectLinuxOnAnotherDisk_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_DetectLinuxOnAnotherDisk_Call) RunAndReturn(run func([]string) sshclient.Output) *Client_DetectLinuxOnAnotherDisk_Call {
	_c.Call.Return(run)
	return _c
}

// DownloadImage provides a mock function with given fields: path, url
func (_m *Client) DownloadImage(path string, url string) sshclient.Output {
	ret := _m.Called(path, url)

	if len(ret) == 0 {
		panic("no return value specified for DownloadImage")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(string, string) sshclient.Output); ok {
		r0 = rf(path, url)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_DownloadImage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DownloadImage'
type Client_DownloadImage_Call struct {
	*mock.Call
}

// DownloadImage is a helper method to define mock.On call
//   - path string
//   - url string
func (_e *Client_Expecter) DownloadImage(path interface{}, url interface{}) *Client_DownloadImage_Call {
	return &Client_DownloadImage_Call{Call: _e.mock.On("DownloadImage", path, url)}
}

func (_c *Client_DownloadImage_Call) Run(run func(path string, url string)) *Client_DownloadImage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string))
	})
	return _c
}

func (_c *Client_DownloadImage_Call) Return(_a0 sshclient.Output) *Client_DownloadImage_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_DownloadImage_Call) RunAndReturn(run func(string, string) sshclient.Output) *Client_DownloadImage_Call {
	_c.Call.Return(run)
	return _c
}

// EnsureCloudInit provides a mock function with given fields:
func (_m *Client) EnsureCloudInit() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for EnsureCloudInit")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_EnsureCloudInit_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'EnsureCloudInit'
type Client_EnsureCloudInit_Call struct {
	*mock.Call
}

// EnsureCloudInit is a helper method to define mock.On call
func (_e *Client_Expecter) EnsureCloudInit() *Client_EnsureCloudInit_Call {
	return &Client_EnsureCloudInit_Call{Call: _e.mock.On("EnsureCloudInit")}
}

func (_c *Client_EnsureCloudInit_Call) Run(run func()) *Client_EnsureCloudInit_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_EnsureCloudInit_Call) Return(_a0 sshclient.Output) *Client_EnsureCloudInit_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_EnsureCloudInit_Call) RunAndReturn(run func() sshclient.Output) *Client_EnsureCloudInit_Call {
	_c.Call.Return(run)
	return _c
}

// ExecuteInstallImage provides a mock function with given fields: hasPostInstallScript
func (_m *Client) ExecuteInstallImage(hasPostInstallScript bool) sshclient.Output {
	ret := _m.Called(hasPostInstallScript)

	if len(ret) == 0 {
		panic("no return value specified for ExecuteInstallImage")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func(bool) sshclient.Output); ok {
		r0 = rf(hasPostInstallScript)
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_ExecuteInstallImage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ExecuteInstallImage'
type Client_ExecuteInstallImage_Call struct {
	*mock.Call
}

// ExecuteInstallImage is a helper method to define mock.On call
//   - hasPostInstallScript bool
func (_e *Client_Expecter) ExecuteInstallImage(hasPostInstallScript interface{}) *Client_ExecuteInstallImage_Call {
	return &Client_ExecuteInstallImage_Call{Call: _e.mock.On("ExecuteInstallImage", hasPostInstallScript)}
}

func (_c *Client_ExecuteInstallImage_Call) Run(run func(hasPostInstallScript bool)) *Client_ExecuteInstallImage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(bool))
	})
	return _c
}

func (_c *Client_ExecuteInstallImage_Call) Return(_a0 sshclient.Output) *Client_ExecuteInstallImage_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_ExecuteInstallImage_Call) RunAndReturn(run func(bool) sshclient.Output) *Client_ExecuteInstallImage_Call {
	_c.Call.Return(run)
	return _c
}

// GetCloudInitOutput provides a mock function with given fields:
func (_m *Client) GetCloudInitOutput() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetCloudInitOutput")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetCloudInitOutput_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCloudInitOutput'
type Client_GetCloudInitOutput_Call struct {
	*mock.Call
}

// GetCloudInitOutput is a helper method to define mock.On call
func (_e *Client_Expecter) GetCloudInitOutput() *Client_GetCloudInitOutput_Call {
	return &Client_GetCloudInitOutput_Call{Call: _e.mock.On("GetCloudInitOutput")}
}

func (_c *Client_GetCloudInitOutput_Call) Run(run func()) *Client_GetCloudInitOutput_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetCloudInitOutput_Call) Return(_a0 sshclient.Output) *Client_GetCloudInitOutput_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetCloudInitOutput_Call) RunAndReturn(run func() sshclient.Output) *Client_GetCloudInitOutput_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUArch provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUArch() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUArch")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUArch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUArch'
type Client_GetHardwareDetailsCPUArch_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUArch is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUArch() *Client_GetHardwareDetailsCPUArch_Call {
	return &Client_GetHardwareDetailsCPUArch_Call{Call: _e.mock.On("GetHardwareDetailsCPUArch")}
}

func (_c *Client_GetHardwareDetailsCPUArch_Call) Run(run func()) *Client_GetHardwareDetailsCPUArch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUArch_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUArch_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUArch_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUArch_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUClockGigahertz provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUClockGigahertz() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUClockGigahertz")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUClockGigahertz_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUClockGigahertz'
type Client_GetHardwareDetailsCPUClockGigahertz_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUClockGigahertz is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUClockGigahertz() *Client_GetHardwareDetailsCPUClockGigahertz_Call {
	return &Client_GetHardwareDetailsCPUClockGigahertz_Call{Call: _e.mock.On("GetHardwareDetailsCPUClockGigahertz")}
}

func (_c *Client_GetHardwareDetailsCPUClockGigahertz_Call) Run(run func()) *Client_GetHardwareDetailsCPUClockGigahertz_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUClockGigahertz_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUClockGigahertz_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUClockGigahertz_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUClockGigahertz_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUCores provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUCores() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUCores")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUCores_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUCores'
type Client_GetHardwareDetailsCPUCores_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUCores is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUCores() *Client_GetHardwareDetailsCPUCores_Call {
	return &Client_GetHardwareDetailsCPUCores_Call{Call: _e.mock.On("GetHardwareDetailsCPUCores")}
}

func (_c *Client_GetHardwareDetailsCPUCores_Call) Run(run func()) *Client_GetHardwareDetailsCPUCores_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUCores_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUCores_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUCores_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUCores_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUFlags provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUFlags() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUFlags")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUFlags_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUFlags'
type Client_GetHardwareDetailsCPUFlags_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUFlags is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUFlags() *Client_GetHardwareDetailsCPUFlags_Call {
	return &Client_GetHardwareDetailsCPUFlags_Call{Call: _e.mock.On("GetHardwareDetailsCPUFlags")}
}

func (_c *Client_GetHardwareDetailsCPUFlags_Call) Run(run func()) *Client_GetHardwareDetailsCPUFlags_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUFlags_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUFlags_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUFlags_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUFlags_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUModel provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUModel() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUModel")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUModel_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUModel'
type Client_GetHardwareDetailsCPUModel_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUModel is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUModel() *Client_GetHardwareDetailsCPUModel_Call {
	return &Client_GetHardwareDetailsCPUModel_Call{Call: _e.mock.On("GetHardwareDetailsCPUModel")}
}

func (_c *Client_GetHardwareDetailsCPUModel_Call) Run(run func()) *Client_GetHardwareDetailsCPUModel_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUModel_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUModel_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUModel_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUModel_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsCPUThreads provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsCPUThreads() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsCPUThreads")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsCPUThreads_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsCPUThreads'
type Client_GetHardwareDetailsCPUThreads_Call struct {
	*mock.Call
}

// GetHardwareDetailsCPUThreads is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsCPUThreads() *Client_GetHardwareDetailsCPUThreads_Call {
	return &Client_GetHardwareDetailsCPUThreads_Call{Call: _e.mock.On("GetHardwareDetailsCPUThreads")}
}

func (_c *Client_GetHardwareDetailsCPUThreads_Call) Run(run func()) *Client_GetHardwareDetailsCPUThreads_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsCPUThreads_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsCPUThreads_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsCPUThreads_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsCPUThreads_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsDebug provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsDebug() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsDebug")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsDebug_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsDebug'
type Client_GetHardwareDetailsDebug_Call struct {
	*mock.Call
}

// GetHardwareDetailsDebug is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsDebug() *Client_GetHardwareDetailsDebug_Call {
	return &Client_GetHardwareDetailsDebug_Call{Call: _e.mock.On("GetHardwareDetailsDebug")}
}

func (_c *Client_GetHardwareDetailsDebug_Call) Run(run func()) *Client_GetHardwareDetailsDebug_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsDebug_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsDebug_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsDebug_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsDebug_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsNics provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsNics() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsNics")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsNics_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsNics'
type Client_GetHardwareDetailsNics_Call struct {
	*mock.Call
}

// GetHardwareDetailsNics is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsNics() *Client_GetHardwareDetailsNics_Call {
	return &Client_GetHardwareDetailsNics_Call{Call: _e.mock.On("GetHardwareDetailsNics")}
}

func (_c *Client_GetHardwareDetailsNics_Call) Run(run func()) *Client_GetHardwareDetailsNics_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsNics_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsNics_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsNics_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsNics_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsRAM provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsRAM() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsRAM")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsRAM_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsRAM'
type Client_GetHardwareDetailsRAM_Call struct {
	*mock.Call
}

// GetHardwareDetailsRAM is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsRAM() *Client_GetHardwareDetailsRAM_Call {
	return &Client_GetHardwareDetailsRAM_Call{Call: _e.mock.On("GetHardwareDetailsRAM")}
}

func (_c *Client_GetHardwareDetailsRAM_Call) Run(run func()) *Client_GetHardwareDetailsRAM_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsRAM_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsRAM_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsRAM_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsRAM_Call {
	_c.Call.Return(run)
	return _c
}

// GetHardwareDetailsStorage provides a mock function with given fields:
func (_m *Client) GetHardwareDetailsStorage() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHardwareDetailsStorage")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHardwareDetailsStorage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHardwareDetailsStorage'
type Client_GetHardwareDetailsStorage_Call struct {
	*mock.Call
}

// GetHardwareDetailsStorage is a helper method to define mock.On call
func (_e *Client_Expecter) GetHardwareDetailsStorage() *Client_GetHardwareDetailsStorage_Call {
	return &Client_GetHardwareDetailsStorage_Call{Call: _e.mock.On("GetHardwareDetailsStorage")}
}

func (_c *Client_GetHardwareDetailsStorage_Call) Run(run func()) *Client_GetHardwareDetailsStorage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHardwareDetailsStorage_Call) Return(_a0 sshclient.Output) *Client_GetHardwareDetailsStorage_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHardwareDetailsStorage_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHardwareDetailsStorage_Call {
	_c.Call.Return(run)
	return _c
}

// GetHostName provides a mock function with given fields:
func (_m *Client) GetHostName() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHostName")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetHostName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetHostName'
type Client_GetHostName_Call struct {
	*mock.Call
}

// GetHostName is a helper method to define mock.On call
func (_e *Client_Expecter) GetHostName() *Client_GetHostName_Call {
	return &Client_GetHostName_Call{Call: _e.mock.On("GetHostName")}
}

func (_c *Client_GetHostName_Call) Run(run func()) *Client_GetHostName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetHostName_Call) Return(_a0 sshclient.Output) *Client_GetHostName_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetHostName_Call) RunAndReturn(run func() sshclient.Output) *Client_GetHostName_Call {
	_c.Call.Return(run)
	return _c
}

// GetRunningInstallImageProcesses provides a mock function with given fields:
func (_m *Client) GetRunningInstallImageProcesses() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRunningInstallImageProcesses")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_GetRunningInstallImageProcesses_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRunningInstallImageProcesses'
type Client_GetRunningInstallImageProcesses_Call struct {
	*mock.Call
}

// GetRunningInstallImageProcesses is a helper method to define mock.On call
func (_e *Client_Expecter) GetRunningInstallImageProcesses() *Client_GetRunningInstallImageProcesses_Call {
	return &Client_GetRunningInstallImageProcesses_Call{Call: _e.mock.On("GetRunningInstallImageProcesses")}
}

func (_c *Client_GetRunningInstallImageProcesses_Call) Run(run func()) *Client_GetRunningInstallImageProcesses_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_GetRunningInstallImageProcesses_Call) Return(_a0 sshclient.Output) *Client_GetRunningInstallImageProcesses_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_GetRunningInstallImageProcesses_Call) RunAndReturn(run func() sshclient.Output) *Client_GetRunningInstallImageProcesses_Call {
	_c.Call.Return(run)
	return _c
}

// Reboot provides a mock function with given fields:
func (_m *Client) Reboot() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Reboot")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_Reboot_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Reboot'
type Client_Reboot_Call struct {
	*mock.Call
}

// Reboot is a helper method to define mock.On call
func (_e *Client_Expecter) Reboot() *Client_Reboot_Call {
	return &Client_Reboot_Call{Call: _e.mock.On("Reboot")}
}

func (_c *Client_Reboot_Call) Run(run func()) *Client_Reboot_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_Reboot_Call) Return(_a0 sshclient.Output) *Client_Reboot_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_Reboot_Call) RunAndReturn(run func() sshclient.Output) *Client_Reboot_Call {
	_c.Call.Return(run)
	return _c
}

// ResetKubeadm provides a mock function with given fields:
func (_m *Client) ResetKubeadm() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ResetKubeadm")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_ResetKubeadm_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ResetKubeadm'
type Client_ResetKubeadm_Call struct {
	*mock.Call
}

// ResetKubeadm is a helper method to define mock.On call
func (_e *Client_Expecter) ResetKubeadm() *Client_ResetKubeadm_Call {
	return &Client_ResetKubeadm_Call{Call: _e.mock.On("ResetKubeadm")}
}

func (_c *Client_ResetKubeadm_Call) Run(run func()) *Client_ResetKubeadm_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_ResetKubeadm_Call) Return(_a0 sshclient.Output) *Client_ResetKubeadm_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_ResetKubeadm_Call) RunAndReturn(run func() sshclient.Output) *Client_ResetKubeadm_Call {
	_c.Call.Return(run)
	return _c
}

// UntarTGZ provides a mock function with given fields:
func (_m *Client) UntarTGZ() sshclient.Output {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for UntarTGZ")
	}

	var r0 sshclient.Output
	if rf, ok := ret.Get(0).(func() sshclient.Output); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(sshclient.Output)
	}

	return r0
}

// Client_UntarTGZ_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UntarTGZ'
type Client_UntarTGZ_Call struct {
	*mock.Call
}

// UntarTGZ is a helper method to define mock.On call
func (_e *Client_Expecter) UntarTGZ() *Client_UntarTGZ_Call {
	return &Client_UntarTGZ_Call{Call: _e.mock.On("UntarTGZ")}
}

func (_c *Client_UntarTGZ_Call) Run(run func()) *Client_UntarTGZ_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Client_UntarTGZ_Call) Return(_a0 sshclient.Output) *Client_UntarTGZ_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Client_UntarTGZ_Call) RunAndReturn(run func() sshclient.Output) *Client_UntarTGZ_Call {
	_c.Call.Return(run)
	return _c
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
