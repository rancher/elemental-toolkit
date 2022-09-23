# gettext in golang

[![Build Status](https://github.com/snapcore/go-gettext/workflows/test/badge.svg)](https://github.com/snapcore/go-gettext/actions)

go-gettext is a pure Go implementation of the GNU Gettext
internationalisation API. It loads the binary catalogs generated
by `msgfmt` by memory mapping them, so there is very little overhead
at startup. Translations are looked up using the data structures in
the catalog directly (either a binary search or hash table lookup).

In addition to the basic `Gettext` API it supports the `NGettext` and
`PGettext` variants, supporting plural translations and translations
requiring a context string respectively.


## Example

```go
import "github.com/snapcore/go-gettext"

domain := &gettext.TextDomain{
	Name:      "messages",
	LocaleDir: "path/to/translations",
}
// or use domain.Locale(lang...) to open a different locale's catalog
locale := domain.UserLocale()

fmt.Println(locale.Gettext("hello from gettext"))

for i := 0; i <= 10; i++ {
	fmt.Printf(locale.NGettext("%d thing\n", "%d things\n", uint32(i)), i)
}
```


## TODO

- [x] parse mofiles
- [x] compile plural forms
- [ ] non-utf8 mo files (possible wontfix)
- [x] gettext
- [x] ngettext
- [x] pgettext/npgettext
- [x] managing mo files / sane API
