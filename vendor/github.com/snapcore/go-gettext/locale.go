package gettext

import (
	"bufio"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
)

// parseLocaleAlias parses a locale.alias file
func parseLocaleAlias(r io.Reader) (map[string]string, error) {
	s := bufio.NewScanner(r)
	aliases := make(map[string]string)
	for s.Scan() {
		// Lines beginning with a hash (after stripping
		// whitespace) are comments
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		words := strings.Fields(line)
		// Ignore lines with fewer than two words
		if len(words) < 2 {
			continue
		}
		// libintl ignores aliases containing an underscore,
		// so we do too.
		if strings.ContainsRune(words[0], '_') {
			continue
		}
		aliases[words[0]] = words[1]
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return aliases, nil
}

var (
	localeAlias     map[string]string
	localeAliasOnce sync.Once
)

// replaceAliases replaces aliased locale names with their canonical versions
func replaceAliases(locale string) string {
	localeAliasOnce.Do(func() {
		f, err := os.Open("/usr/share/locale/locale.alias")
		if err != nil {
			if !os.IsNotExist(err) {
				log.Println("Can not open locale.alias:", err)
			}
			return
		}
		defer f.Close()
		localeAlias, err = parseLocaleAlias(f)
		if err != nil {
			log.Println("Can not parse locale.alias:", err)
		}
	})

	if replacement, ok := localeAlias[locale]; ok {
		return replacement
	}
	return locale
}

var (
	notAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	allDigits   = regexp.MustCompile(`^[0-9]*$`)
)

// normalizeCodeset produces a normalized version of a codeset
// Based on libintl's _nl_normalize_codeset
func normalizeCodeset(codeset string) string {
	codeset = strings.ToLower(notAlphaNum.ReplaceAllLiteralString(codeset, ""))
	if allDigits.MatchString(codeset) {
		codeset = "iso" + codeset
	}
	return "." + codeset
}

const (
	xpgNormCodeset = 1 << iota
	xpgCodeset
	xpgTerritory
	xpgModifier
)

func expandLocale(locale string) (locales []string) {
	locale = replaceAliases(locale)

	// Split the locale based on libintl's _nl_explode_name()
	var language, territory, codeset, normCodeset, modifier string
	mask := 0
	pos := strings.IndexRune(locale, '@')
	if pos != -1 {
		modifier = locale[pos:]
		locale = locale[:pos]
		mask |= xpgModifier
	}
	pos = strings.IndexRune(locale, '.')
	if pos != -1 {
		codeset = locale[pos:]
		locale = locale[:pos]
		mask |= xpgCodeset

		normCodeset = normalizeCodeset(codeset)
		if normCodeset != "" && normCodeset != codeset {
			mask |= xpgNormCodeset
		}
	}
	pos = strings.IndexRune(locale, '_')
	if pos != -1 {
		territory = locale[pos:]
		locale = locale[:pos]
		mask |= xpgTerritory
	}
	language = locale

	for i := mask; i >= 0; i-- {
		if i&^mask != 0 {
			// Only consider cases where all components
			// are available
			continue
		}
		if i&xpgNormCodeset != 0 && i&xpgCodeset != 0 {
			// Don't include both versions of the codeset
			continue
		}
		locale := language
		if i&xpgTerritory != 0 {
			locale += territory
		}
		if i&xpgCodeset != 0 {
			locale += codeset
		}
		if i&xpgNormCodeset != 0 {
			locale += normCodeset
		}
		if i&xpgModifier != 0 {
			locale += modifier
		}
		locales = append(locales, locale)
	}
	return locales
}

// normalizeLanguages expands a list of locales to include fallbacks
// and remove duplicates.
func normalizeLanguages(locales []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, singleLocale := range locales {
		for _, locale := range expandLocale(singleLocale) {
			if locale == "C" || locale == "POSIX" {
				// These special locales identifiers
				// indicate no translation.  We don't
				// include them in the list, and
				// ignore any further locales.
				return result
			}
			if seen[locale] {
				continue
			}
			seen[locale] = true
			result = append(result, locale)
		}
	}
	return result
}

var osGetenv = os.Getenv

// UserLanguages returns a list of the user's preferred languages
//
// These are in the form of POSIX locale identifiers.  This lookup is
// based on the logic of libintl's guess_category_value()
func UserLanguages() []string {
	// libintl uses $LANGUAGE by default as a colon-separated list
	// of locale names.
	locale := osGetenv("LANGUAGE")
	if len(locale) != 0 {
		return strings.Split(locale, ":")
	}

	// It falls back to POSIX locale's LC_MESSAGES category, which
	// is controlled by the first of $LC_ALL, $LC_MESSAGES, and
	// $LANG.  Each of those contains a single locale.
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		locale = osGetenv(env)
		if len(locale) != 0 {
			return []string{locale}
		}
	}

	// libintl also includes some platform specific fallbacks for
	// Windows and MacOS that we have not implemented.
	return nil
}
