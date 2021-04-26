package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"text/template"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Values struct {
	Flavors []string
}

type Flavor struct {
	Flavor string
}

func main(){
	buildTemplate, err := ioutil.ReadFile("raw-workflows/build.tmpl")
	check(err)
	buildTemplateParsed, err := template.New("").Delims("[[", "]]").Parse(string(buildTemplate))
	check(err)

	valuesFile, err := ioutil.ReadFile("raw-workflows/values.yaml")
	check(err)
	valuesParsed := Values{}
	err = yaml.Unmarshal(valuesFile, &valuesParsed)
	check(err)

	for _, flavor := range valuesParsed.Flavors {
		fmt.Printf("Generating files for flavor: %v\n", flavor)
		f := Flavor{Flavor: flavor}
		fileName := fmt.Sprintf("workflows/build-%s.yaml", flavor)
		file, err := os.Create(fileName)
		err = buildTemplateParsed.Execute(file, f)
		check(err)
		err = file.Close()
		check(err)
		fmt.Printf("Done Generating files for flavor: %s\n", flavor)
	}
}
