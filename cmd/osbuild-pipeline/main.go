package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

func main() {
	var format string
	var blueprintArg string
	var distroArg string
	flag.StringVar(&format, "output-format", "qcow2", "output format")
	flag.StringVar(&blueprintArg, "blueprint", "", "blueprint to translate")
	flag.StringVar(&distroArg, "distro", "", "distribution to create")
	flag.Parse()

	blueprint := &blueprint.Blueprint{}
	if blueprintArg != "" {
		file, err := ioutil.ReadFile(blueprintArg)
		if err != nil {
			panic("Colud not find blueprint")
		}
		err = json.Unmarshal([]byte(file), &blueprint)
		if err != nil {
			panic("Colud not parse blueprint")
		}
	}

	d := distro.New(distroArg)
	if d == nil {
		panic("unknown distro: " + distroArg)
	}

	pipeline, err := d.Pipeline(blueprint, format)
	if err != nil {
		panic(err.Error())
	}

	bytes, err := json.Marshal(pipeline)
	if err != nil {
		panic("could not marshal pipeline into JSON")
	}

	os.Stdout.Write(bytes)
}
