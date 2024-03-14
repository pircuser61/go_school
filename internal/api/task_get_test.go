package api

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
)

func TestGetAccessibleForms(t *testing.T) {
	tests := []struct {
		Name            string
		CurrentUse      string
		Steps           *entity.TaskSteps
		Delegates       *ht.Delegations
		WantErr         bool
		AccessibleForms map[string]struct{}
	}{
		{
			Name:            "test valid blocks with empty Steps",
			CurrentUse:      "",
			Delegates:       nil,
			Steps:           &entity.TaskSteps{},
			WantErr:         false,
			AccessibleForms: map[string]struct{}{},
		},
		{
			Name:            "test valid blocks with empty Steps and Delegates",
			CurrentUse:      "",
			Delegates:       &ht.Delegations{},
			Steps:           &entity.TaskSteps{},
			WantErr:         false,
			AccessibleForms: map[string]struct{}{},
		},
		{
			Name:       "test with accessibleForms error unmarshal",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_err.json"),
			WantErr:    true,
		},
		{
			Name:       "test with accessibleForms step - form_0",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_1.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
			},
		},
		{
			Name:       "test accessibleForms step - form_1",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_2.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_1": {},
			},
		},
		{
			Name:       "test accessibleForms step - approve_0 (form_0 - None, form_1 - ReadWrite)",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_3.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_1": {},
			},
		},
		{
			Name:       "test accessibleForms step - form_0",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_4.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_1": {},
			},
		},
		{
			Name:       "test accessibleForms step - form_0 - diffrent user",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_7.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_1": {},
			},
		},
		{
			Name:       "test accessibleForms step - form_1 (form_0 - ReadWrite)",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_5.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_1": {},
			},
		},
		{
			Name:       "test with accessibleForms step - approve_0 (form_1 - ReadWrite, form_2 - ReadWrite)",
			CurrentUse: "user1",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_6.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_1": {},
			},
		},
		{
			Name:       "test with accessibleForms step - form0,1,2,3 -  approve_0 - execution_0",
			CurrentUse: "user2",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_8.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_2": {},
				"form_3": {},
			},
		},
		{
			Name:       "test with accessibleForms step - form0,1,2 - form_3 -  sign_0 - execution_0",
			CurrentUse: "user2",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_9.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_1": {},
				"form_2": {},
				"form_3": {},
			},
		},
		{
			Name:       "test with accessibleForms step - form0,1,2 - form_3 -  sign_0 - execution_0 - initial_executor",
			CurrentUse: "user3",
			Delegates:  &ht.Delegations{},
			Steps:      unmarshalStepFromTestFile(t, "testdata/steps_get_accessible_forms_9.json"),
			WantErr:    false,
			AccessibleForms: map[string]struct{}{
				"form_0": {},
				"form_3": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ae := &Env{}
			accessibleForms, ttErr := ae.getAccessibleForms(tt.CurrentUse, tt.Steps, tt.Delegates)

			assert.Equalf(t, tt.WantErr, ttErr != nil, "getAccessibleForms(%v, %v, %v)", tt.CurrentUse, tt.Steps, tt.Delegates)
			assert.Equalf(t, tt.AccessibleForms, accessibleForms, "getAccessibleForms(%v, %v, %v)", tt.CurrentUse, tt.Steps, tt.Delegates)
		})
	}
}

func unmarshalStepFromTestFile(t *testing.T, in string) *entity.TaskSteps {
	bytes, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}

	var result entity.TaskSteps

	err = json.Unmarshal(bytes, &result)
	if err != nil {
		t.Fatal(err)
	}

	return &result
}
