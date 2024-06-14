package script

import (
	"errors"
	"fmt"
	"strings"
)

type ApproverType string

func (a ApproverType) String() string {
	return string(a)
}

type ApprovementRule string

func (a ApprovementRule) String() string {
	return string(a)
}

type AutoAction string

const (
	SettingStatusApprovement     = "На согласовании"
	SettingStatusApproveConfirm  = "На утверждении"
	SettingStatusApproveView     = "На ознакомлении"
	SettingStatusApproveInform   = "На информировании"
	SettingStatusApproveSign     = "На подписании"
	SettingStatusApproveSignUkep = "На подписании УКЭП"

	ApproverTypeUser       ApproverType = "user"
	ApproverTypeGroup      ApproverType = "group"
	ApproverTypeHead       ApproverType = "head"
	ApproverTypeFromSchema ApproverType = "fromSchema"

	AllOfApprovementRequired ApprovementRule = "AllOf"
	AnyOfApprovementRequired ApprovementRule = "AnyOf"
)

type ApproverParams struct {
	Type            ApproverType `json:"type"`
	ApprovementRule `json:"approvementRule"`
	Approver        string `json:"approver"`

	SLA                int                 `json:"sla"`
	CheckSLA           bool                `json:"check_sla"`
	ReworkSLA          int                 `json:"rework_sla"`
	CheckReworkSLA     bool                `json:"check_rework_sla"`
	AutoAction         *string             `json:"auto_action,omitempty"`
	FormsAccessibility []FormAccessibility `json:"forms_accessibility"`

	IsEditable           bool    `json:"is_editable"`
	RepeatPrevDecision   bool    `json:"repeat_prev_decision"`
	ApproversGroupID     string  `json:"approvers_group_id"`
	ApproversGroupName   string  `json:"approvers_group_name"`
	ApproversGroupIDPath *string `json:"approvers_group_id_path,omitempty"`
	ApproveStatusName    string  `json:"approve_status_name"`
	WorkType             *string `json:"work_type"`
	WaitAllDecisions     bool    `json:"wait_all"`
}

func (a *ApproverParams) Validate() error {
	if a.Approver == "" && a.Type == ApproverTypeUser {
		return errors.New("approver is empty")
	}

	if (a.ApproversGroupID == "" && a.ApproversGroupIDPath == nil) && a.Type == ApproverTypeGroup {
		return errors.New("approvers group is empty")
	}

	typeApprove := ApproverType(a.Type.String())

	err := a.validateApproverType(typeApprove)
	if err != nil {
		return err
	}

	if a.Type == ApproverTypeFromSchema &&
		len(strings.Split(a.Approver, ";")) > 1 &&
		a.ApprovementRule == "" {
		return errors.New("approvement rule is empty")
	}

	if a.Type == ApproverTypeGroup && a.ApprovementRule == "" {
		return errors.New("approvement rule is empty")
	}

	if a.CheckSLA && a.SLA <= 0 {
		return fmt.Errorf("invalid SLA: %d", a.SLA)
	}

	if a.IsEditable && a.CheckReworkSLA && a.ReworkSLA < 16 {
		return fmt.Errorf("invalid rework SLA: %d", a.SLA)
	}

	return nil
}

func (a *ApproverParams) validateApproverType(typeApprove ApproverType) error {
	if typeApprove != ApproverTypeUser &&
		typeApprove != ApproverTypeGroup &&
		typeApprove != ApproverTypeHead &&
		typeApprove != ApproverTypeFromSchema {
		return fmt.Errorf("unknown approver type: %s", a.Type)
	}

	return nil
}
