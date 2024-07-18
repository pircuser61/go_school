package pipeline

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	en "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func TestApproverData_getFinalGroupDecision(t *testing.T) {
	type fields struct {
		Type                         script.ApproverType
		Approvers                    map[string]struct{}
		Decision                     *ApproverDecision
		DecisionAttachments          []en.Attachment
		Comment                      *string
		ActualApprover               *string
		ApprovementRule              script.ApprovementRule
		ApproverLog                  []ApproverLogEntry
		WaitAllDecisions             bool
		IsExpired                    bool
		IsEditable                   bool
		RepeatPrevDecision           bool
		EditingApp                   *ApproverEditingApp
		EditingAppLog                []ApproverEditingApp
		FormsAccessibility           []script.FormAccessibility
		ApproversGroupID             string
		ApproversGroupName           string
		ApproversGroupIDPath         *string
		AddInfo                      []AdditionalInfo
		ApproveStatusName            string
		Deadline                     time.Time
		SLA                          int
		CheckSLA                     bool
		SLAChecked                   bool
		HalfSLAChecked               bool
		ReworkSLA                    int
		CheckReworkSLA               bool
		CheckDayBeforeSLARequestInfo bool
		WorkType                     string
		AutoAction                   *ApproverAction
		ActionList                   []Action
		AdditionalApprovers          []AdditionalApprover
	}
	type args struct {
		ds ApproverDecision
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		wantFinalDecision ApproverDecision
		wantIsFinal       bool
	}{
		{
			name: "wait all decisions, final rejected",
			fields: fields{
				Type: "",
				Approvers: map[string]struct{}{
					"user1": {},
					"user2": {},
					"user3": {},
				},
				Decision:        nil,
				ApprovementRule: script.AllOfApprovementRequired,
				ApproverLog: []ApproverLogEntry{
					{
						Login:    "user1",
						Decision: ApproverDecisionApproved,
						Comment:  "test comment user 1",
						LogType:  ApproverLogDecision,
					},
					{
						Login:    "user2",
						Decision: ApproverDecisionRejected,
						Comment:  "test comment user 2",
						LogType:  ApproverLogDecision,
					},
					{
						Login:    "user3",
						Decision: ApproverDecisionApproved,
						Comment:  "test comment user 3",
						LogType:  ApproverLogDecision,
					},
				},
				WaitAllDecisions: true,
			},
			args:              args{ds: ApproverDecisionApproved},
			wantFinalDecision: ApproverDecisionRejected,
			wantIsFinal:       true,
		},
		{
			name: "wait all decisions, not final yet",
			fields: fields{
				Type: "",
				Approvers: map[string]struct{}{
					"user1": {},
					"user2": {},
					"user3": {},
				},
				Decision:        nil,
				ApprovementRule: script.AllOfApprovementRequired,
				ApproverLog: []ApproverLogEntry{
					{
						Login:    "user1",
						Decision: ApproverDecisionRejected,
						Comment:  "test comment user 1",
						LogType:  ApproverLogDecision,
					},
				},
				WaitAllDecisions: true,
			},
			args:              args{ds: ApproverDecisionRejected},
			wantFinalDecision: "",
			wantIsFinal:       false,
		},
		{
			name: "reject decision immediately",
			fields: fields{
				Type: "",
				Approvers: map[string]struct{}{
					"user1": {},
					"user2": {},
					"user3": {},
				},
				Decision:        nil,
				ApprovementRule: script.AllOfApprovementRequired,
				ApproverLog: []ApproverLogEntry{
					{
						Login:    "user1",
						Decision: ApproverDecisionApproved,
						Comment:  "test comment user 1",
						LogType:  ApproverLogDecision,
					},
				},
				WaitAllDecisions: false,
			},
			args:              args{ds: ApproverDecisionRejected},
			wantFinalDecision: ApproverDecisionRejected,
			wantIsFinal:       true,
		},
		{
			name: "sent to edit decision",
			fields: fields{
				Type: "",
				Approvers: map[string]struct{}{
					"user1": {},
					"user2": {},
					"user3": {},
				},
				Decision:        nil,
				ApprovementRule: script.AllOfApprovementRequired,
				ApproverLog: []ApproverLogEntry{
					{
						Login:    "user1",
						Decision: ApproverDecisionApproved,
						Comment:  "test comment user 1",
						LogType:  ApproverLogDecision,
					},
					{
						Login:    "user2",
						Decision: ApproverDecisionApproved,
						Comment:  "test comment user 2",
						LogType:  ApproverLogDecision,
					},
					{
						Login:    "user3",
						Decision: ApproverDecisionSentToEdit,
						Comment:  "need fix application",
						LogType:  ApproverLogDecision,
					},
				},
				WaitAllDecisions: true,
			},
			args:              args{ds: ApproverDecisionSentToEdit},
			wantFinalDecision: ApproverDecisionSentToEdit,
			wantIsFinal:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverData{
				Type:                         tt.fields.Type,
				Approvers:                    tt.fields.Approvers,
				Decision:                     tt.fields.Decision,
				DecisionAttachments:          tt.fields.DecisionAttachments,
				Comment:                      tt.fields.Comment,
				ActualApprover:               tt.fields.ActualApprover,
				ApprovementRule:              tt.fields.ApprovementRule,
				ApproverLog:                  tt.fields.ApproverLog,
				WaitAllDecisions:             tt.fields.WaitAllDecisions,
				IsExpired:                    tt.fields.IsExpired,
				IsEditable:                   tt.fields.IsEditable,
				RepeatPrevDecision:           tt.fields.RepeatPrevDecision,
				EditingApp:                   tt.fields.EditingApp,
				EditingAppLog:                tt.fields.EditingAppLog,
				FormsAccessibility:           tt.fields.FormsAccessibility,
				ApproversGroupID:             tt.fields.ApproversGroupID,
				ApproversGroupName:           tt.fields.ApproversGroupName,
				ApproversGroupIDPath:         tt.fields.ApproversGroupIDPath,
				AddInfo:                      tt.fields.AddInfo,
				ApproveStatusName:            tt.fields.ApproveStatusName,
				Deadline:                     tt.fields.Deadline,
				SLA:                          tt.fields.SLA,
				CheckSLA:                     tt.fields.CheckSLA,
				SLAChecked:                   tt.fields.SLAChecked,
				HalfSLAChecked:               tt.fields.HalfSLAChecked,
				ReworkSLA:                    tt.fields.ReworkSLA,
				CheckReworkSLA:               tt.fields.CheckReworkSLA,
				CheckDayBeforeSLARequestInfo: tt.fields.CheckDayBeforeSLARequestInfo,
				WorkType:                     tt.fields.WorkType,
				AutoAction:                   tt.fields.AutoAction,
				ActionList:                   tt.fields.ActionList,
				AdditionalApprovers:          tt.fields.AdditionalApprovers,
			}
			gotFinalDecision, gotIsFinal := a.getFinalGroupDecision(tt.args.ds)
			assert.Equalf(t, tt.wantFinalDecision, gotFinalDecision, "getFinalGroupDecision(%v)", tt.args.ds)
			assert.Equalf(t, tt.wantIsFinal, gotIsFinal, "getFinalGroupDecision(%v)", tt.args.ds)
		})
	}
}

func TestApproverData_calculateDecisions(t *testing.T) {
	type fields struct {
		Type                         script.ApproverType
		Approvers                    map[string]struct{}
		Decision                     *ApproverDecision
		DecisionAttachments          []en.Attachment
		Comment                      *string
		ActualApprover               *string
		ApprovementRule              script.ApprovementRule
		ApproverLog                  []ApproverLogEntry
		WaitAllDecisions             bool
		IsExpired                    bool
		IsEditable                   bool
		RepeatPrevDecision           bool
		EditingApp                   *ApproverEditingApp
		EditingAppLog                []ApproverEditingApp
		FormsAccessibility           []script.FormAccessibility
		ApproversGroupID             string
		ApproversGroupName           string
		ApproversGroupIDPath         *string
		AddInfo                      []AdditionalInfo
		ApproveStatusName            string
		Deadline                     time.Time
		SLA                          int
		CheckSLA                     bool
		SLAChecked                   bool
		HalfSLAChecked               bool
		ReworkSLA                    int
		CheckReworkSLA               bool
		CheckDayBeforeSLARequestInfo bool
		WorkType                     string
		AutoAction                   *ApproverAction
		ActionList                   []Action
		AdditionalApprovers          []AdditionalApprover
	}
	tests := []struct {
		name              string
		fields            fields
		wantIsFinal       bool
		wantRejectExist   bool
		wantSendEditExist bool
		wantP             map[ApproverDecision]int
	}{
		{
			name: "is final",
			fields: fields{
				ApproverLog: []ApproverLogEntry{
					{
						Decision: ApproverDecisionRejected,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionConfirmed,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionViewed,
						LogType:  ApproverLogDecision,
					},
				},
				Approvers: map[string]struct{}{
					"ebalnik":   {},
					"kozaegaza": {},
					"pholty":    {},
				},
			},
			wantIsFinal:       true,
			wantRejectExist:   true,
			wantSendEditExist: false,
			wantP: map[ApproverDecision]int{
				ApproverDecisionConfirmed: 1,
				ApproverDecisionViewed:    1,
			},
		},
		{
			name: "not final",
			fields: fields{
				ApproverLog: []ApproverLogEntry{
					{
						LogType: ApproverLogAddApprover,
					},
					{
						Decision: ApproverDecisionConfirmed,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionViewed,
						LogType:  ApproverLogDecision,
					},
				},
				Approvers: map[string]struct{}{
					"ebalnik":   {},
					"kozaegaza": {},
					"pholty":    {},
				},
			},
			wantIsFinal:       false,
			wantRejectExist:   false,
			wantSendEditExist: false,
			wantP: map[ApproverDecision]int{
				ApproverDecisionConfirmed: 1,
				ApproverDecisionViewed:    1,
			},
		},
		{
			name: "wantRejectExist and wantSendEditExist",
			fields: fields{
				ApproverLog: []ApproverLogEntry{
					{
						LogType: ApproverLogAddApprover,
					},
					{
						Decision: ApproverDecisionRejected,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionSentToEdit,
						LogType:  ApproverLogDecision,
					},
				},
				Approvers: map[string]struct{}{
					"ebalnik":   {},
					"kozaegaza": {},
					"pholty":    {},
				},
			},
			wantIsFinal:       false,
			wantRejectExist:   true,
			wantSendEditExist: true,
			wantP:             map[ApproverDecision]int{},
		},
		{
			name: "send to edit",
			fields: fields{
				ApproverLog: []ApproverLogEntry{
					{
						Decision: ApproverDecisionApproved,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionApproved,
						LogType:  ApproverLogDecision,
					},
					{
						Decision: ApproverDecisionSentToEdit,
						LogType:  ApproverLogDecision,
					},
				},
				Approvers: map[string]struct{}{
					"ebalnik":   {},
					"kozaegaza": {},
					"pholty":    {},
				},
			},
			wantIsFinal:       true,
			wantRejectExist:   false,
			wantSendEditExist: true,
			wantP:             map[ApproverDecision]int{ApproverDecisionApproved: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverData{
				Type:                         tt.fields.Type,
				Approvers:                    tt.fields.Approvers,
				Decision:                     tt.fields.Decision,
				DecisionAttachments:          tt.fields.DecisionAttachments,
				Comment:                      tt.fields.Comment,
				ActualApprover:               tt.fields.ActualApprover,
				ApprovementRule:              tt.fields.ApprovementRule,
				ApproverLog:                  tt.fields.ApproverLog,
				WaitAllDecisions:             tt.fields.WaitAllDecisions,
				IsExpired:                    tt.fields.IsExpired,
				IsEditable:                   tt.fields.IsEditable,
				RepeatPrevDecision:           tt.fields.RepeatPrevDecision,
				EditingApp:                   tt.fields.EditingApp,
				EditingAppLog:                tt.fields.EditingAppLog,
				FormsAccessibility:           tt.fields.FormsAccessibility,
				ApproversGroupID:             tt.fields.ApproversGroupID,
				ApproversGroupName:           tt.fields.ApproversGroupName,
				ApproversGroupIDPath:         tt.fields.ApproversGroupIDPath,
				AddInfo:                      tt.fields.AddInfo,
				ApproveStatusName:            tt.fields.ApproveStatusName,
				Deadline:                     tt.fields.Deadline,
				SLA:                          tt.fields.SLA,
				CheckSLA:                     tt.fields.CheckSLA,
				SLAChecked:                   tt.fields.SLAChecked,
				HalfSLAChecked:               tt.fields.HalfSLAChecked,
				ReworkSLA:                    tt.fields.ReworkSLA,
				CheckReworkSLA:               tt.fields.CheckReworkSLA,
				CheckDayBeforeSLARequestInfo: tt.fields.CheckDayBeforeSLARequestInfo,
				WorkType:                     tt.fields.WorkType,
				AutoAction:                   tt.fields.AutoAction,
				ActionList:                   tt.fields.ActionList,
				AdditionalApprovers:          tt.fields.AdditionalApprovers,
			}
			gotIsFinal, gotRejectExist, gotSendEditExist, gotP := a.calculateDecisions()
			if gotIsFinal != tt.wantIsFinal {
				t.Errorf("calculateDecisions() gotIsFinal = %v, want %v", gotIsFinal, tt.wantIsFinal)
			}
			if gotRejectExist != tt.wantRejectExist {
				t.Errorf("calculateDecisions() gotRejectExist = %v, want %v", gotRejectExist, tt.wantRejectExist)
			}
			if gotSendEditExist != tt.wantSendEditExist {
				t.Errorf("calculateDecisions() gotSendEditExist = %v, want %v", gotSendEditExist, tt.wantSendEditExist)
			}
			if !reflect.DeepEqual(gotP, tt.wantP) {
				t.Errorf("calculateDecisions() gotP = %v, want %v", gotP, tt.wantP)
			}
		})
	}
}
