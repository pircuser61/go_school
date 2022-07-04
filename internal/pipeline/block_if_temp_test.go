package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestIF_ProcessConditions(t *testing.T) {
	jsonBytes, err := ioutil.ReadFile("testdata/jsontest.json")
	if err != nil {
		fmt.Print(err)
	}

	var params interface{}
	if unmarshalErr := json.Unmarshal(jsonBytes, &params); unmarshalErr != nil {
		t.Error("Cannot unmarshal test schema json.")
	}
	fmt.Print("kek")
}
