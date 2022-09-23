// Implements gettext in pure Go with Plural Forms support.

package gettext

import (
	"fmt"
	"os"
	"path"
	"sync"
)

// TextDomain represents a collection of translatable strings.
//
// The Locale and UserLocale methods can be used to access
// translations of those strings in various languages.
type TextDomain struct {
	// Name is the name of the text domain
	Name string
	// LocaleDir is the base directory holding translations of the
	// domain.  If it is empty, DefaultLocaleDir will be used.
	LocaleDir string
	// PathResolver is called to determine the path of a
	// particular locale's translations.  If it is nil then
	// DefaultResolver will be used, which implements the standard
	// gettext directory layout.
	PathResolver PathResolver

	mu    sync.Mutex
	cache map[string]*mocatalog
}

const DefaultLocaleDir = "/usr/share/locale"

// PathResolver resolves a path to a mo file
type PathResolver func(root string, locale string, domain string) string

// DefaultResolver resolves paths in the standard format of:
// <root>/<locale>/LC_MESSAGES/<domain>.mo
func DefaultResolver(root string, locale string, domain string) string {
	return path.Join(root, locale, "LC_MESSAGES", fmt.Sprintf("%s.mo", domain))
}

// Preload a list of locales (if they're available). This is useful if you want
// to limit IO to a specific time in your app, for example startup. Subsequent
// calls to Preload or Locale using a locale given here will not do any IO.
func (t *TextDomain) Preload(locales ...string) {
	for _, locale := range locales {
		t.load(locale)
	}
}

func (t *TextDomain) load(locale string) *mocatalog {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cache == nil {
		t.cache = make(map[string]*mocatalog)
	}

	if catalog, ok := t.cache[locale]; ok {
		return catalog
	}

	localeDir := t.LocaleDir
	if localeDir == "" {
		localeDir = DefaultLocaleDir
	}
	resolver := t.PathResolver
	if resolver == nil {
		resolver = DefaultResolver
	}
	t.cache[locale] = nil
	path := resolver(localeDir, locale, t.Name)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	catalog, err := parseMO(f)
	if err != nil {
		return nil
	}
	t.cache[locale] = catalog
	return catalog
}

// Locale returns the catalog translations for a list of locales.
//
// If translations are not found in the first locale, the each
// subsequent one is consulted until a match is found.  If no match is
// found, the original strings are returned.
func (t *TextDomain) Locale(languages ...string) Catalog {
	var mos []*mocatalog
	for _, lang := range normalizeLanguages(languages) {
		mo := t.load(lang)
		if mo != nil {
			mos = append(mos, mo)
		}
	}
	return Catalog{mos}
}

// UserLocale returns the catalog translations for the user's Locale.
func (t *TextDomain) UserLocale() Catalog {
	return t.Locale(UserLanguages()...)
}

// Translations is an alias for a TextDomain pointer
//
// Deprecated: this type alias is provided for backwards
// compatibility.  New code should use TextDomain directly.
type Translations = *TextDomain

// NewTranslations initialises a TextDomain struct, setting the Name,
// LocaleDir and PathResolver fields.
//
// Deprecated: New code should initialise TextDomain directly.
func NewTranslations(localeDir, domain string, resolver PathResolver) Translations {
	return &TextDomain{
		Name:         domain,
		LocaleDir:    localeDir,
		PathResolver: resolver,
	}
}
