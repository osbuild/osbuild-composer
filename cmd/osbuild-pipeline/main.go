package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"

	"github.com/osbuild/osbuild-composer/internal/distro"
	_ "github.com/osbuild/osbuild-composer/internal/distro/fedora30"
)

func main() {
	var format string
	var blueprintArg string
	flag.StringVar(&format, "output-format", "qcow2", "output format")
	flag.StringVar(&blueprintArg, "blueprint", "", "blueprint to translate")
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

	f30 := distro.New("fedora-30")
	pipeline, err := f30.Pipeline(blueprint, format)
	if err != nil {
		panic(err.Error())
	}

	bytes, err := json.Marshal(pipeline)
	if err != nil {
		panic("could not marshal pipeline into JSON")
	}

	os.Stdout.Write(bytes)
}
