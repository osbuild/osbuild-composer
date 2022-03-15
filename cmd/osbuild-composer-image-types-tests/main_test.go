package main

import (
    "fmt"
    "os/exec"
    "testing"
)

func TestImageTypes(t *testing.T) {
        cmd := exec.Command("composer-cli", "compose", "types")
        stdout, err := cmd.Output()

        if err != nil {                                                                                                               fmt.Println(err.Error())
                return
        }

        // Print the output
        fmt.Println(string(stdout))                                                                                   

	t.Errorf(string(stdout))
}
