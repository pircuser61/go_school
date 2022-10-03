// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	context "context"

	db "gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	entity "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	mock "github.com/stretchr/testify/mock"

	orderedmap "github.com/iancoleman/orderedmap"

	time "time"

	uuid "github.com/google/uuid"
)

// MockedDatabase is an autogenerated mock type for the Database type
type MockedDatabase struct {
	mock.Mock
}

// ActiveAlertNGSA provides a mock function with given fields: ctx, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc
func (_m *MockedDatabase) ActiveAlertNGSA(ctx context.Context, sever int, state string, source string, eventType string, cause string, addInf string, addTxt string, moID string, specProb string, notID string, usertext string, moi string, moc string) error {
	ret := _m.Called(ctx, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, string, string, string, string, string, string, string, string, string, string, string, string) error); ok {
		r0 = rf(ctx, sever, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AttachTag provides a mock function with given fields: ctx, id, p
func (_m *MockedDatabase) AttachTag(ctx context.Context, id uuid.UUID, p *entity.EriusTagInfo) error {
	ret := _m.Called(ctx, id, p)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, *entity.EriusTagInfo) error); ok {
		r0 = rf(ctx, id, p)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ChangeTaskStatus provides a mock function with given fields: ctx, taskID, status
func (_m *MockedDatabase) ChangeTaskStatus(ctx context.Context, taskID uuid.UUID, status int) error {
	ret := _m.Called(ctx, taskID, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, int) error); ok {
		r0 = rf(ctx, taskID, status)
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

// ClearAlertNGSA provides a mock function with given fields: ctx, name
func (_m *MockedDatabase) ClearAlertNGSA(ctx context.Context, name string) error {
	ret := _m.Called(ctx, name)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, name)
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

// CreateTag provides a mock function with given fields: ctx, e, author
func (_m *MockedDatabase) CreateTag(ctx context.Context, e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error) {
	ret := _m.Called(ctx, e, author)

	var r0 *entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo, string) *entity.EriusTagInfo); ok {
		r0 = rf(ctx, e, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *entity.EriusTagInfo, string) error); ok {
		r1 = rf(ctx, e, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateTask provides a mock function with given fields: ctx, dto
func (_m *MockedDatabase) CreateTask(ctx context.Context, dto *db.CreateTaskDTO) (*entity.EriusTask, error) {
	ret := _m.Called(ctx, dto)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, *db.CreateTaskDTO) *entity.EriusTask); ok {
		r0 = rf(ctx, dto)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *db.CreateTaskDTO) error); ok {
		r1 = rf(ctx, dto)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateVersion provides a mock function with given fields: ctx, p, author, pipelineData
func (_m *MockedDatabase) CreateVersion(ctx context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	ret := _m.Called(ctx, p, author, pipelineData)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusScenario, string, []byte) error); ok {
		r0 = rf(ctx, p, author, pipelineData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteAllVersions provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) DeleteAllVersions(ctx context.Context, id uuid.UUID) error {
	ret := _m.Called(ctx, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeletePipeline provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) DeletePipeline(ctx context.Context, id uuid.UUID) error {
	ret := _m.Called(ctx, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteVersion provides a mock function with given fields: ctx, versionID
func (_m *MockedDatabase) DeleteVersion(ctx context.Context, versionID uuid.UUID) error {
	ret := _m.Called(ctx, versionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(ctx, versionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DetachTag provides a mock function with given fields: ctx, id, p
func (_m *MockedDatabase) DetachTag(ctx context.Context, id uuid.UUID, p *entity.EriusTagInfo) error {
	ret := _m.Called(ctx, id, p)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, *entity.EriusTagInfo) error); ok {
		r0 = rf(ctx, id, p)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// EditTag provides a mock function with given fields: ctx, e
func (_m *MockedDatabase) EditTag(ctx context.Context, e *entity.EriusTagInfo) error {
	ret := _m.Called(ctx, e)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo) error); ok {
		r0 = rf(ctx, e)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetAllTags provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetAllTags(ctx context.Context) ([]entity.EriusTagInfo, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusTagInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetApplicationData provides a mock function with given fields: workNumber
func (_m *MockedDatabase) GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error) {
	ret := _m.Called(workNumber)

	var r0 *orderedmap.OrderedMap
	if rf, ok := ret.Get(0).(func(string) *orderedmap.OrderedMap); ok {
		r0 = rf(workNumber)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*orderedmap.OrderedMap)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(workNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetApprovedVersions provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetApprovedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDraftVersions provides a mock function with given fields: ctx, author
func (_m *MockedDatabase) GetDraftVersions(ctx context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx, author)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context, string) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutableByName provides a mock function with given fields: ctx, name
func (_m *MockedDatabase) GetExecutableByName(ctx context.Context, name string) (*entity.EriusScenario, error) {
	ret := _m.Called(ctx, name)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.EriusScenario); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutableScenarios provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetExecutableScenarios(ctx context.Context) ([]entity.EriusScenario, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenario); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLastDebugTask provides a mock function with given fields: ctx, versionID, author
func (_m *MockedDatabase) GetLastDebugTask(ctx context.Context, versionID uuid.UUID, author string) (*entity.EriusTask, error) {
	ret := _m.Called(ctx, versionID, author)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) *entity.EriusTask); ok {
		r0 = rf(ctx, versionID, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, versionID, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetOnApproveVersions provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetOnApproveVersions(ctx context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetParentTaskStepByName provides a mock function with given fields: ctx, workID, stepName
func (_m *MockedDatabase) GetParentTaskStepByName(ctx context.Context, workID uuid.UUID, stepName string) (*entity.Step, error) {
	ret := _m.Called(ctx, workID, stepName)

	var r0 *entity.Step
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) *entity.Step); ok {
		r0 = rf(ctx, workID, stepName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.Step)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, workID, stepName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipeline provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetPipeline(ctx context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	ret := _m.Called(ctx, id)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusScenario); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
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

// GetPipelineTag provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetPipelineTag(ctx context.Context, id uuid.UUID) ([]entity.EriusTagInfo, error) {
	ret := _m.Called(ctx, id)

	var r0 []entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) []entity.EriusTagInfo); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusTagInfo)
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

// GetPipelineTasks provides a mock function with given fields: ctx, pipelineID
func (_m *MockedDatabase) GetPipelineTasks(ctx context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	ret := _m.Called(ctx, pipelineID)

	var r0 *entity.EriusTasks
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusTasks); ok {
		r0 = rf(ctx, pipelineID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, pipelineID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipelineVersion provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetPipelineVersion(ctx context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	ret := _m.Called(ctx, id)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusScenario); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
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

// GetPipelineVersions provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetPipelineVersions(ctx context.Context, id uuid.UUID) ([]entity.EriusVersionInfo, error) {
	ret := _m.Called(ctx, id)

	var r0 []entity.EriusVersionInfo
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) []entity.EriusVersionInfo); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusVersionInfo)
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

// GetPipelinesByNameOrId provides a mock function with given fields: ctx, dto
func (_m *MockedDatabase) GetPipelinesByNameOrId(ctx context.Context, dto *db.SearchPipelineRequest) ([]entity.SearchPipeline, error) {
	ret := _m.Called(ctx, dto)

	var r0 []entity.SearchPipeline
	if rf, ok := ret.Get(0).(func(context.Context, *db.SearchPipelineRequest) []entity.SearchPipeline); ok {
		r0 = rf(ctx, dto)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.SearchPipeline)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *db.SearchPipelineRequest) error); ok {
		r1 = rf(ctx, dto)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPipelinesWithLatestVersion provides a mock function with given fields: ctx, author
func (_m *MockedDatabase) GetPipelinesWithLatestVersion(ctx context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx, author)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context, string) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRejectedVersions provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetRejectedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTag provides a mock function with given fields: ctx, e
func (_m *MockedDatabase) GetTag(ctx context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error) {
	ret := _m.Called(ctx, e)

	var r0 *entity.EriusTagInfo
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusTagInfo) *entity.EriusTagInfo); ok {
		r0 = rf(ctx, e)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTagInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *entity.EriusTagInfo) error); ok {
		r1 = rf(ctx, e)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTask provides a mock function with given fields: ctx, workNumber
func (_m *MockedDatabase) GetTask(ctx context.Context, workNumber string) (*entity.EriusTask, error) {
	ret := _m.Called(ctx, workNumber)

	var r0 *entity.EriusTask
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.EriusTask); ok {
		r0 = rf(ctx, workNumber)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, workNumber)
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

// GetTaskStepByName provides a mock function with given fields: ctx, workID, stepName
func (_m *MockedDatabase) GetTaskStepByName(ctx context.Context, workID uuid.UUID, stepName string) (*entity.Step, error) {
	ret := _m.Called(ctx, workID, stepName)

	var r0 *entity.Step
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) *entity.Step); ok {
		r0 = rf(ctx, workID, stepName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.Step)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, workID, stepName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTaskSteps provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) GetTaskSteps(ctx context.Context, id uuid.UUID) (entity.TaskSteps, error) {
	ret := _m.Called(ctx, id)

	var r0 entity.TaskSteps
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) entity.TaskSteps); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(entity.TaskSteps)
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

// GetTasks provides a mock function with given fields: ctx, filters
func (_m *MockedDatabase) GetTasks(ctx context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	ret := _m.Called(ctx, filters)

	var r0 *entity.EriusTasksPage
	if rf, ok := ret.Get(0).(func(context.Context, entity.TaskFilter) *entity.EriusTasksPage); ok {
		r0 = rf(ctx, filters)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasksPage)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, entity.TaskFilter) error); ok {
		r1 = rf(ctx, filters)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTasksCount provides a mock function with given fields: ctx, userName
func (_m *MockedDatabase) GetTasksCount(ctx context.Context, userName string) (*entity.CountTasks, error) {
	ret := _m.Called(ctx, userName)

	var r0 *entity.CountTasks
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.CountTasks); ok {
		r0 = rf(ctx, userName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.CountTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, userName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUnfinishedTaskStepsByWorkIdAndStepType provides a mock function with given fields: ctx, id, stepType
func (_m *MockedDatabase) GetUnfinishedTaskStepsByWorkIdAndStepType(ctx context.Context, id uuid.UUID, stepType string) (entity.TaskSteps, error) {
	ret := _m.Called(ctx, id, stepType)

	var r0 entity.TaskSteps
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) entity.TaskSteps); ok {
		r0 = rf(ctx, id, stepType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(entity.TaskSteps)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, id, stepType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUnfinishedTasks provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetUnfinishedTasks(ctx context.Context) (*entity.EriusTasks, error) {
	ret := _m.Called(ctx)

	var r0 *entity.EriusTasks
	if rf, ok := ret.Get(0).(func(context.Context) *entity.EriusTasks); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionByWorkNumber provides a mock function with given fields: ctx, workNumber
func (_m *MockedDatabase) GetVersionByWorkNumber(ctx context.Context, workNumber string) (*entity.EriusScenario, error) {
	ret := _m.Called(ctx, workNumber)

	var r0 *entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, string) *entity.EriusScenario); ok {
		r0 = rf(ctx, workNumber)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, workNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionTasks provides a mock function with given fields: ctx, versionID
func (_m *MockedDatabase) GetVersionTasks(ctx context.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	ret := _m.Called(ctx, versionID)

	var r0 *entity.EriusTasks
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *entity.EriusTasks); ok {
		r0 = rf(ctx, versionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*entity.EriusTasks)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, versionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionsByPipelineID provides a mock function with given fields: ctx, blueprintID
func (_m *MockedDatabase) GetVersionsByPipelineID(ctx context.Context, blueprintID string) ([]entity.EriusScenario, error) {
	ret := _m.Called(ctx, blueprintID)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context, string) []entity.EriusScenario); ok {
		r0 = rf(ctx, blueprintID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, blueprintID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVersionsByStatus provides a mock function with given fields: ctx, status, author
func (_m *MockedDatabase) GetVersionsByStatus(ctx context.Context, status int, author string) ([]entity.EriusScenarioInfo, error) {
	ret := _m.Called(ctx, status, author)

	var r0 []entity.EriusScenarioInfo
	if rf, ok := ret.Get(0).(func(context.Context, int, string) []entity.EriusScenarioInfo); ok {
		r0 = rf(ctx, status, author)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenarioInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int, string) error); ok {
		r1 = rf(ctx, status, author)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkedVersions provides a mock function with given fields: ctx
func (_m *MockedDatabase) GetWorkedVersions(ctx context.Context) ([]entity.EriusScenario, error) {
	ret := _m.Called(ctx)

	var r0 []entity.EriusScenario
	if rf, ok := ret.Get(0).(func(context.Context) []entity.EriusScenario); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]entity.EriusScenario)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PipelineNameCreatable provides a mock function with given fields: ctx, name
func (_m *MockedDatabase) PipelineNameCreatable(ctx context.Context, name string) (bool, error) {
	ret := _m.Called(ctx, name)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PipelineRemovable provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) PipelineRemovable(ctx context.Context, id uuid.UUID) (bool, error) {
	ret := _m.Called(ctx, id)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) bool); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemovePipelineTags provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) RemovePipelineTags(ctx context.Context, id uuid.UUID) error {
	ret := _m.Called(ctx, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveTag provides a mock function with given fields: ctx, id
func (_m *MockedDatabase) RemoveTag(ctx context.Context, id uuid.UUID) error {
	ret := _m.Called(ctx, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RenamePipeline provides a mock function with given fields: ctx, id, name
func (_m *MockedDatabase) RenamePipeline(ctx context.Context, id uuid.UUID, name string) error {
	ret := _m.Called(ctx, id, name)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) error); ok {
		r0 = rf(ctx, id, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RollbackVersion provides a mock function with given fields: ctx, pipelineID, versionID
func (_m *MockedDatabase) RollbackVersion(ctx context.Context, pipelineID uuid.UUID, versionID uuid.UUID) error {
	ret := _m.Called(ctx, pipelineID, versionID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID) error); ok {
		r0 = rf(ctx, pipelineID, versionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveStepContext provides a mock function with given fields: ctx, dto
func (_m *MockedDatabase) SaveStepContext(ctx context.Context, dto *db.SaveStepRequest) (uuid.UUID, time.Time, error) {
	ret := _m.Called(ctx, dto)

	var r0 uuid.UUID
	if rf, ok := ret.Get(0).(func(context.Context, *db.SaveStepRequest) uuid.UUID); ok {
		r0 = rf(ctx, dto)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(uuid.UUID)
		}
	}

	var r1 time.Time
	if rf, ok := ret.Get(1).(func(context.Context, *db.SaveStepRequest) time.Time); ok {
		r1 = rf(ctx, dto)
	} else {
		r1 = ret.Get(1).(time.Time)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, *db.SaveStepRequest) error); ok {
		r2 = rf(ctx, dto)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// SetApplicationData provides a mock function with given fields: workNumber, data
func (_m *MockedDatabase) SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error {
	ret := _m.Called(workNumber, data)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, *orderedmap.OrderedMap) error); ok {
		r0 = rf(workNumber, data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SwitchApproved provides a mock function with given fields: ctx, pipelineID, versionID, author
func (_m *MockedDatabase) SwitchApproved(ctx context.Context, pipelineID uuid.UUID, versionID uuid.UUID, author string) error {
	ret := _m.Called(ctx, pipelineID, versionID, author)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID, string) error); ok {
		r0 = rf(ctx, pipelineID, versionID, author)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SwitchRejected provides a mock function with given fields: ctx, versionID, comment, author
func (_m *MockedDatabase) SwitchRejected(ctx context.Context, versionID uuid.UUID, comment string, author string) error {
	ret := _m.Called(ctx, versionID, comment, author)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string, string) error); ok {
		r0 = rf(ctx, versionID, comment, author)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateDraft provides a mock function with given fields: ctx, p, pipelineData
func (_m *MockedDatabase) UpdateDraft(ctx context.Context, p *entity.EriusScenario, pipelineData []byte) error {
	ret := _m.Called(ctx, p, pipelineData)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *entity.EriusScenario, []byte) error); ok {
		r0 = rf(ctx, p, pipelineData)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateStepContext provides a mock function with given fields: ctx, dto
func (_m *MockedDatabase) UpdateStepContext(ctx context.Context, dto *db.UpdateStepRequest) error {
	ret := _m.Called(ctx, dto)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *db.UpdateStepRequest) error); ok {
		r0 = rf(ctx, dto)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateTaskBlocksData provides a mock function with given fields: ctx, dto
func (_m *MockedDatabase) UpdateTaskBlocksData(ctx context.Context, dto *db.UpdateTaskBlocksDataRequest) error {
	ret := _m.Called(ctx, dto)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *db.UpdateTaskBlocksDataRequest) error); ok {
		r0 = rf(ctx, dto)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateTaskHumanStatus provides a mock function with given fields: ctx, taskID, status
func (_m *MockedDatabase) UpdateTaskHumanStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	ret := _m.Called(ctx, taskID, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) error); ok {
		r0 = rf(ctx, taskID, status)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VersionEditable provides a mock function with given fields: ctx, versionID
func (_m *MockedDatabase) VersionEditable(ctx context.Context, versionID uuid.UUID) (bool, error) {
	ret := _m.Called(ctx, versionID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) bool); ok {
		r0 = rf(ctx, versionID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, versionID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockedDatabase interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockedDatabase creates a new instance of MockedDatabase. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockedDatabase(t mockConstructorTestingTNewMockedDatabase) *MockedDatabase {
	mock := &MockedDatabase{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
