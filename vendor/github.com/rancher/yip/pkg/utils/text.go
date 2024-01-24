package utils

import (
	"bytes"
	"math/rand"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// UUIDTemplatedString accepts a template, and renders it with a
// UUID v4 generated string.
// E.g. input "foo-{{.}}"
func TemplatedString(t string, i interface{}) (string, error) {
	b := bytes.NewBuffer([]byte{})
	tmpl, err := template.New("template").Funcs(sprig.TxtFuncMap()).Parse(t)
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(b, i)

	return b.String(), err
}

var letters = []rune("1234567890abcdefghijklmnopqrstuvwxyz")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
