// Code generated by mockery v2.4.0. DO NOT EDIT.

package mocks

import (
	context "context"

	db "gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	entity "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	mock "github.com/stretchr/testify/mock"

	time "time"

	uuid "github.com/google/uuid"
)

// MockedDatabase is an autogenerated mock type for the Database type
type MockedDatabase struct {
	mock.Mock
}

// ActiveAlertNGSA provides a mock function with given fields: c, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc
func (_m *MockedDatabase) ActiveAlertNGSA(c context.Context, sever int, state string, source string, eventType string, cause string, addInf string, addTxt string, moID string, specProb string, notID string, usertext string, moi string, moc string) error {
	ret := _m.Called(c, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, string, string, string, string, string, string, string, string, string, string, string, string) error); ok {
		r0 = rf(c, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AttachTag provides a mock function with given fields: c, p, e
func (_m *MockedDatabase) AttachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error {
	ret := _m.Called(c, p, e)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, *entity.EriusTagInfo) error); ok {
		r0 = rf(c, p, e)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ChangeTaskStatus provides a mock function with given fields: c, taskID, status
func (_m *MockedDatabase) ChangeTaskStatus(c context.Context, taskID uuid.UUID, status int) error {
	ret := _m.Called(c, taskID, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, int) error); ok {
		r0 = rf(c, taskID, status)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CheckTaskStepsExecuted provides a mock function with given fields: ctx, workNumber, blocks
func (_m *MockedDatabase) CheckTaskStepsExecuted(ctx context.Context, workNumber string, blocks []string) (bool, error) {
	ret := _m.Called(ctx, workNumber, blocks)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, string, []string) bool); ok {
		r0 = rf(ctx, workNumber, blocks)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []string) error); ok {
		r1 = rf(ctx, workNumber, blocks)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClearAlertNGSA provides a mock function with given fields: c, name
func (_m *MockedDatabase) ClearAlertNGSA(c context.Context, name string) error {
	ret := _m.Called(c, name)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(c, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreatePipeline provides a mock function with given fields: c, p, author, pipelineData
func (_m *MockedDatabase) CreatePipeline(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	ret := _m.Called(c, p, author, pipelineData)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusScenario, string, []byte) error); ok {
		r0 = rf(c, p, author, pipelineData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateTag provides a mock function with given fields: c, e, author
func (_m *MockedDatabase) CreateTag(c context.Context, e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error) {
	ret := _m.Called(c, e, author)

	var r0 *entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo, string) *entity.EriusTagInfo); ok {
		r0 = rf(c, e, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *entity.EriusTagInfo, string) error); ok {
		r1 = rf(c, e, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateTask provides a mock function with given fields: c, taskID, versionID, author, isDebugMode, parameters
func (_m *MockedDatabase) CreateTask(c context.Context, taskID uuid.UUID, versionID uuid.UUID, author string, isDebugMode bool, parameters []byte) (*entity.EriusTask, error) {
	ret := _m.Called(c, taskID, versionID, author, isDebugMode, parameters)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID, string, bool, []byte) *entity.EriusTask); ok {
		r0 = rf(c, taskID, versionID, author, isDebugMode, parameters)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, uuid.UUID, string, bool, []byte) error); ok {
		r1 = rf(c, taskID, versionID, author, isDebugMode, parameters)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateVersion provides a mock function with given fields: c, p, author, pipelineData
func (_m *MockedDatabase) CreateVersion(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	ret := _m.Called(c, p, author, pipelineData)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusScenario, string, []byte) error); ok {
		r0 = rf(c, p, author, pipelineData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteAllVersions provides a mock function with given fields: c, id
func (_m *MockedDatabase) DeleteAllVersions(c context.Context, id uuid.UUID) error {
	ret := _m.Called(c, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(c, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeletePipeline provides a mock function with given fields: c, id
func (_m *MockedDatabase) DeletePipeline(c context.Context, id uuid.UUID) error {
	ret := _m.Called(c, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(c, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteVersion provides a mock function with given fields: c, versionID
func (_m *MockedDatabase) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	ret := _m.Called(c, versionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(c, versionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DetachTag provides a mock function with given fields: c, p, e
func (_m *MockedDatabase) DetachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error {
	ret := _m.Called(c, p, e)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, *entity.EriusTagInfo) error); ok {
		r0 = rf(c, p, e)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DraftPipelineCreatable provides a mock function with given fields: c, id, author
func (_m *MockedDatabase) DraftPipelineCreatable(c context.Context, id uuid.UUID, author string) (bool, error) {
	ret := _m.Called(c, id, author)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) bool); ok {
		r0 = rf(c, id, author)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(c, id, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// EditTag provides a mock function with given fields: c, e
func (_m *MockedDatabase) EditTag(c context.Context, e *entity.EriusTagInfo) error {
	ret := _m.Called(c, e)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo) error); ok {
		r0 = rf(c, e)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetAllTags provides a mock function with given fields: c
func (_m *MockedDatabase) GetAllTags(c context.Context) ([]entity.EriusTagInfo, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusTagInfo); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetApprovedVersions provides a mock function with given fields: c
func (_m *MockedDatabase) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDraftVersions provides a mock function with given fields: c
func (_m *MockedDatabase) GetDraftVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutableByName provides a mock function with given fields: c, name
func (_m *MockedDatabase) GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error) {
	ret := _m.Called(c, name)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.EriusScenario); ok {
		r0 = rf(c, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(c, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutableScenarios provides a mock function with given fields: c
func (_m *MockedDatabase) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenario); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLastDebugTask provides a mock function with given fields: c, versionID, author
func (_m *MockedDatabase) GetLastDebugTask(c context.Context, versionID uuid.UUID, author string) (*entity.EriusTask, error) {
	ret := _m.Called(c, versionID, author)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) *entity.EriusTask); ok {
		r0 = rf(c, versionID, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(c, versionID, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetOnApproveVersions provides a mock function with given fields: c
func (_m *MockedDatabase) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipeline provides a mock function with given fields: c, id
func (_m *MockedDatabase) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	ret := _m.Called(c, id)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusScenario); ok {
		r0 = rf(c, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipelineTag provides a mock function with given fields: c, id
func (_m *MockedDatabase) GetPipelineTag(c context.Context, id uuid.UUID) ([]entity.EriusTagInfo, error) {
	ret := _m.Called(c, id)

	var r0 []entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) []entity.EriusTagInfo); ok {
		r0 = rf(c, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipelineTasks provides a mock function with given fields: c, pipelineID
func (_m *MockedDatabase) GetPipelineTasks(c context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	ret := _m.Called(c, pipelineID)

	var r0 *entity.EriusTasks
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusTasks); ok {
		r0 = rf(c, pipelineID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, pipelineID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipelineVersion provides a mock function with given fields: c, id
func (_m *MockedDatabase) GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	ret := _m.Called(c, id)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusScenario); ok {
		r0 = rf(c, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRejectedVersions provides a mock function with given fields: c
func (_m *MockedDatabase) GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTag provides a mock function with given fields: c, e
func (_m *MockedDatabase) GetTag(c context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error) {
	ret := _m.Called(c, e)

	var r0 *entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo) *entity.EriusTagInfo); ok {
		r0 = rf(c, e)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *entity.EriusTagInfo) error); ok {
		r1 = rf(c, e)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTask provides a mock function with given fields: c, workNumber
func (_m *MockedDatabase) GetTask(c context.Context, workNumber string) (*entity.EriusTask, error) {
	ret := _m.Called(c, workNumber)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.EriusTask); ok {
		r0 = rf(c, workNumber)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(c, workNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTaskStepById provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	ret := _m.Called(ctx, id)

	var r0 *entity.Step
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.Step); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.Step)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTaskSteps provides a mock function with given fields: c, id
func (_m *MockedDatabase) GetTaskSteps(c context.Context, id uuid.UUID) (entity.TaskSteps, error) {
	ret := _m.Called(c, id)

	var r0 entity.TaskSteps
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) entity.TaskSteps); ok {
		r0 = rf(c, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(entity.TaskSteps)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTasks provides a mock function with given fields: c, filters
func (_m *MockedDatabase) GetTasks(c context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	ret := _m.Called(c, filters)

	var r0 *entity.EriusTasksPage
	if rf, ok := ret.Get(0).(func(context.Context, entity.TaskFilter) *entity.EriusTasksPage); ok {
		r0 = rf(c, filters)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasksPage)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, entity.TaskFilter) error); ok {
		r1 = rf(c, filters)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *MockedDatabase) GetTasksCount(c context.Context, userName string) (*entity.CountTasks, error) {
	ret := _m.Called(c, userName)

	var r0 *entity.CountTasks
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.CountTasks); ok {
		r0 = rf(c, userName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.CountTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(c, userName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUnfinishedTaskStepsByWorkIdAndStepType provides a mock function with given fields: c, id, stepType
func (_m *MockedDatabase) GetUnfinishedTaskStepsByWorkIdAndStepType(c context.Context, id uuid.UUID, stepType string) (entity.TaskSteps, error) {
	ret := _m.Called(c, id, stepType)

	var r0 entity.TaskSteps
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) entity.TaskSteps); ok {
		r0 = rf(c, id, stepType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(entity.TaskSteps)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(c, id, stepType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionTasks provides a mock function with given fields: c, versionID
func (_m *MockedDatabase) GetVersionTasks(c context.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	ret := _m.Called(c, versionID)

	var r0 *entity.EriusTasks
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusTasks); ok {
		r0 = rf(c, versionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, versionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionsByBlueprintID provides a mock function with given fields: c, blueprintID
func (_m *MockedDatabase) GetVersionsByBlueprintID(c context.Context, blueprintID string) ([]entity.EriusScenario, error) {
	ret := _m.Called(c, blueprintID)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, string) []entity.EriusScenario); ok {
		r0 = rf(c, blueprintID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(c, blueprintID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionsByStatus provides a mock function with given fields: c, status
func (_m *MockedDatabase) GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(c, status)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context, int) []entity.EriusScenarioInfo); ok {
		r0 = rf(c, status)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(c, status)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkedVersions provides a mock function with given fields: c
func (_m *MockedDatabase) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	ret := _m.Called(c)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenario); ok {
		r0 = rf(c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PipelineNameCreatable provides a mock function with given fields: c, name
func (_m *MockedDatabase) PipelineNameCreatable(c context.Context, name string) (bool, error) {
	ret := _m.Called(c, name)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(c, name)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(c, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PipelineRemovable provides a mock function with given fields: c, id
func (_m *MockedDatabase) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	ret := _m.Called(c, id)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) bool); ok {
		r0 = rf(c, id)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemovePipelineTags provides a mock function with given fields: c, id
func (_m *MockedDatabase) RemovePipelineTags(c context.Context, id uuid.UUID) error {
	ret := _m.Called(c, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(c, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveTag provides a mock function with given fields: c, id
func (_m *MockedDatabase) RemoveTag(c context.Context, id uuid.UUID) error {
	ret := _m.Called(c, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(c, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RollbackVersion provides a mock function with given fields: c, pipelineID, versionID
func (_m *MockedDatabase) RollbackVersion(c context.Context, pipelineID uuid.UUID, versionID uuid.UUID) error {
	ret := _m.Called(c, pipelineID, versionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID) error); ok {
		r0 = rf(c, pipelineID, versionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveStepContext provides a mock function with given fields: c, dto
func (_m *MockedDatabase) SaveStepContext(c context.Context, dto *db.SaveStepRequest) (uuid.UUID, time.Time, error) {
	ret := _m.Called(c, dto)

	var r0 uuid.UUID
	if rf, ok := ret.Get(0).(func(context.Context, *db.SaveStepRequest) uuid.UUID); ok {
		r0 = rf(c, dto)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(uuid.UUID)
		}
	}

	var r1 time.Time
	if rf, ok := ret.Get(1).(func(context.Context, *db.SaveStepRequest) time.Time); ok {
		r1 = rf(c, dto)
	} else {
		r1 = ret.Get(1).(time.Time)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, *db.SaveStepRequest) error); ok {
		r2 = rf(c, dto)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// SwitchApproved provides a mock function with given fields: c, pipelineID, versionID, author
func (_m *MockedDatabase) SwitchApproved(c context.Context, pipelineID uuid.UUID, versionID uuid.UUID, author string) error {
	ret := _m.Called(c, pipelineID, versionID, author)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID, string) error); ok {
		r0 = rf(c, pipelineID, versionID, author)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SwitchRejected provides a mock function with given fields: c, versionID, comment, author
func (_m *MockedDatabase) SwitchRejected(c context.Context, versionID uuid.UUID, comment string, author string) error {
	ret := _m.Called(c, versionID, comment, author)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string, string) error); ok {
		r0 = rf(c, versionID, comment, author)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateDraft provides a mock function with given fields: c, p, pipelineData
func (_m *MockedDatabase) UpdateDraft(c context.Context, p *entity.EriusScenario, pipelineData []byte) error {
	ret := _m.Called(c, p, pipelineData)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusScenario, []byte) error); ok {
		r0 = rf(c, p, pipelineData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateStepContext provides a mock function with given fields: c, dto
func (_m *MockedDatabase) UpdateStepContext(c context.Context, dto *db.UpdateStepRequest) error {
	ret := _m.Called(c, dto)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *db.UpdateStepRequest) error); ok {
		r0 = rf(c, dto)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateTaskHumanStatus provides a mock function with given fields: c, taskID, status
func (_m *MockedDatabase) UpdateTaskHumanStatus(c context.Context, taskID uuid.UUID, status string) error {
	ret := _m.Called(c, taskID, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) error); ok {
		r0 = rf(c, taskID, status)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VersionEditable provides a mock function with given fields: c, versionID
func (_m *MockedDatabase) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	ret := _m.Called(c, versionID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) bool); ok {
		r0 = rf(c, versionID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(c, versionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
