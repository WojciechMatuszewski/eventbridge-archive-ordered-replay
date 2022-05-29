package env

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/multierr"
)

type Variable string

const (
	EVENT_BUS_ARCHIVE_NAME Variable = "eventBusArchiveName"
	EVENT_BUS_ARCHIVE_ARN  Variable = "eventBusArchiveArn"

	EVENT_BUS_NAME Variable = "eventBusName"
	EVENT_BUS_ARN  Variable = "eventBusArn"

	REPLAY_RULE_ROLE_ARN     Variable = "replayRuleRoleArn"
	REPLAY_STATE_MACHINE_ARN Variable = "replayStateMachineArn"
	REPLAY_RULE_NAME         Variable = "replayRuleName"
)

func Get(variable Variable) (string, error) {
	outputsPath, err := getOutputsPath()
	if err != nil {
		return "", err
	}

	outputs, err := readOutputs(outputsPath)
	if err != nil {
		return "", err
	}

	if variable == EVENT_BUS_ARCHIVE_NAME {
		return outputs["InfraStack"].EventBusArchiveName, nil
	}

	if variable == EVENT_BUS_ARCHIVE_ARN {
		return outputs["InfraStack"].EventBusArchiveArn, nil
	}

	if variable == EVENT_BUS_NAME {
		return outputs["InfraStack"].EventBusName, nil
	}

	if variable == EVENT_BUS_ARN {
		return outputs["InfraStack"].EventBusArn, nil
	}

	if variable == REPLAY_RULE_ROLE_ARN {
		return outputs["InfraStack"].ReplayRuleRoleArn, nil
	}

	if variable == REPLAY_STATE_MACHINE_ARN {
		return outputs["InfraStack"].ReplayStateMachineArn, nil
	}

	if variable == REPLAY_RULE_NAME {
		rawName := outputs["InfraStack"].ReplayRuleName
		// For some reason, the output is `EVENT_BRIDGE_NAME|REPLAY_RULE_NAME` and not the expected `REPLAY_RULE_NAME`
		// I have no idea why CDK is formatting the output like that.
		_, after, found := strings.Cut(rawName, "|")
		if !found {
			return "", fmt.Errorf("unable to parse replay rule name: %s", rawName)
		}
		return after, nil
	}

	return "", fmt.Errorf("unknown variable: %s", variable)
}

type StackOutputs struct {
	EventBusArchiveName string `json:"eventBusArchiveName"`
	EventBusArchiveArn  string `json:"eventBusArchiveArn"`

	EventBusName string `json:"eventBusName"`
	EventBusArn  string `json:"eventBusArn"`

	ReplayRuleRoleArn     string `json:"replayRuleRoleArn"`
	ReplayStateMachineArn string `json:"replayStateMachineArn"`
	ReplayRuleName        string `json:"replayRuleName"`
}

type Outputs map[string]StackOutputs

func readOutputs(outputsPath string) (_ Outputs, err error) {
	fd, err := os.Open(outputsPath)
	if err != nil {
		return Outputs{}, err
	}
	defer multierr.AppendInvoke(&err, multierr.Close(fd))

	var outputs Outputs
	err = json.NewDecoder(fd).Decode(&outputs)
	if err != nil {
		return Outputs{}, err
	}

	return outputs, nil
}

func getOutputsPath() (string, error) {
	rootPath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	var outputsPath string
	err = filepath.WalkDir(rootPath, func(path string, dirEntry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dirEntry.IsDir() && dirEntry.Name() == "outputs.json" {
			outputsPath = path
			return io.EOF
		}

		return nil
	})
	if err != io.EOF {
		return "", err
	}

	return outputsPath, nil
}
