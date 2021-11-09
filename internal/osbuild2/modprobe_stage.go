package osbuild2

import (
	"encoding/json"
	"fmt"
)

type ModprobeStageOptions struct {
	Filename string                `json:"filename"`
	Commands ModprobeConfigCmdList `json:"commands"`
}

func (ModprobeStageOptions) isStageOptions() {}

func NewModprobeStage(options *ModprobeStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.modprobe",
		Options: options,
	}
}

type ModprobeConfigCmd interface {
	isModprobeConfigCmd()
}

// ModprobeConfigCmdList represents a modprobe configuration file, which contains
// a list of commands.
type ModprobeConfigCmdList []ModprobeConfigCmd

func (configFile *ModprobeConfigCmdList) UnmarshalJSON(data []byte) error {
	var rawConfigFile []interface{}

	if err := json.Unmarshal(data, &rawConfigFile); err != nil {
		return err
	}

	for _, rawConfigCmd := range rawConfigFile {
		var modprobeCmd ModprobeConfigCmd

		// The command object structure depends on the value of "command"
		// item, therefore make no assumptions on the structure.
		configCmdMap, ok := rawConfigCmd.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected modprobe configuration file format")
		}
		command, ok := configCmdMap["command"].(string)
		if !ok {
			return fmt.Errorf("'command' item should be string, not %T", configCmdMap["command"])
		}

		switch command {
		case "blacklist":
			modulename, ok := configCmdMap["modulename"].(string)
			if !ok {
				return fmt.Errorf("'modulename' item should be string, not %T", configCmdMap["modulename"])
			}
			modprobeCmd = NewModprobeConfigCmdBlacklist(modulename)
		case "install":
			modulename, ok := configCmdMap["modulename"].(string)
			if !ok {
				return fmt.Errorf("'modulename' item should be string, not %T", configCmdMap["modulename"])
			}
			cmdline, ok := configCmdMap["cmdline"].(string)
			if !ok {
				return fmt.Errorf("'cmdline' item should be string, not %T", configCmdMap["cmdline"])
			}
			modprobeCmd = NewModprobeConfigCmdInstall(modulename, cmdline)
		default:
			return fmt.Errorf("unexpected modprobe command: %s", command)
		}

		*configFile = append(*configFile, modprobeCmd)
	}

	return nil
}

func (o ModprobeConfigCmdList) MarshalJSON() ([]byte, error) {
	if len(o) == 0 {
		return nil, fmt.Errorf("at least one modprobe command must be specified for a configuration file")
	}
	var configList []ModprobeConfigCmd = o
	return json.Marshal(configList)
}

// ModprobeConfigCmdBlacklist represents the 'blacklist' command in the
// modprobe configuration.
type ModprobeConfigCmdBlacklist struct {
	Command    string `json:"command"`
	Modulename string `json:"modulename"`
}

func (ModprobeConfigCmdBlacklist) isModprobeConfigCmd() {}

// NewModprobeConfigCmdBlacklist creates a new instance of ModprobeConfigCmdBlacklist
// for the provided modulename.
func NewModprobeConfigCmdBlacklist(modulename string) *ModprobeConfigCmdBlacklist {
	return &ModprobeConfigCmdBlacklist{
		Command:    "blacklist",
		Modulename: modulename,
	}
}

// ModprobeConfigCmdInstall represents the 'install' command in the
// modprobe configuration.
type ModprobeConfigCmdInstall struct {
	Command    string `json:"command"`
	Modulename string `json:"modulename"`
	Cmdline    string `json:"cmdline"`
}

func (ModprobeConfigCmdInstall) isModprobeConfigCmd() {}

// NewModprobeConfigCmdInstall creates a new instance of ModprobeConfigCmdInstall
// for the provided modulename.
func NewModprobeConfigCmdInstall(modulename, cmdline string) *ModprobeConfigCmdInstall {
	return &ModprobeConfigCmdInstall{
		Command:    "install",
		Modulename: modulename,
		Cmdline:    cmdline,
	}
}
