package entity

import "reflect"

type NodeEvent struct {
	TaskID        string                 `json:"task_id"`
	WorkNumber    string                 `json:"work_number"`
	NodeName      string                 `json:"node_name"`
	NodeShortName string                 `json:"node_short_name"`
	NodeStart     string                 `json:"node_start"`
	NodeEnd       string                 `json:"node_end"`
	TaskStatus    string                 `json:"task_status"`
	NodeStatus    string                 `json:"node_status"`
	NodeOutput    map[string]interface{} `json:"node_output"`
}

func (ne *NodeEvent) ToMap() map[string]interface{} {
	if ne.NodeOutput == nil {
		ne.NodeOutput = make(map[string]interface{})
	}

	res := make(map[string]interface{})

	for i := 0; i < reflect.TypeOf(*ne).NumField(); i++ {
		f := reflect.TypeOf(*ne).Field(i)
		k := f.Tag.Get("json")

		if k == "" {
			continue
		}

		val := reflect.ValueOf(*ne).Field(i).Interface()
		res[k] = val
	}

	return res
}

type ToSendKafkaEvent struct {
	EventID string
	Event   NodeKafkaEvent
}

type NodeKafkaEvent struct {
	TaskID           string                 `json:"task_id"`
	WorkNumber       string                 `json:"work_number"`
	NodeName         string                 `json:"node_name"`
	NodeShortName    string                 `json:"node_short_name"`
	NodeStart        int64                  `json:"node_start"`
	NodeEnd          int64                  `json:"node_end"`
	TaskStatus       string                 `json:"task_status"`
	NodeStatus       string                 `json:"node_status"`
	Initiator        string                 `json:"initiator"`
	CreatedAt        int64                  `json:"created_at"`
	NodeSLA          int64                  `json:"node_sla"`
	Action           string                 `json:"action"`
	NodeType         string                 `json:"node_type"`
	ActionBody       map[string]interface{} `json:"action_body"`
	AvailableActions []string               `json:"available_actions"`
}
