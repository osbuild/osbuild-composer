package main

import (
	"encoding/json"
	"flag"
	"os"
	"osbuild-composer/internal/blueprint"
)

func main() {
	var format string
	flag.StringVar(&format, "output-format", "qcow2", "output format")
	flag.Parse()

	blueprint := &blueprint.Blueprint{}
	pipeline, err := blueprint.ToPipeline(format)
	if err != nil {
		panic(err.Error())
	}

	bytes, err := json.Marshal(pipeline)
	if err != nil {
		panic("could not marshal pipeline into JSON")
	}

	os.Stdout.Write(bytes)
}
