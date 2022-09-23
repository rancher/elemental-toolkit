package gettext

// Catalog of translations for a given locale.
type Catalog struct {
	mos []*mocatalog
}

func (c Catalog) findMsg(msgid string, usePlural bool, n uint32) (msgstr string, ok bool) {
	for _, mo := range c.mos {
		if msgstr, ok := mo.findMsg(msgid, usePlural, n); ok {
			return msgstr, true
		}
	}
	return "", false
}

// Gettext returns a translation of the provided message.
//
// If no translation is available, the original message is returned.
func (c Catalog) Gettext(msgid string) string {
	if msgstr, ok := c.findMsg(msgid, false, 0); ok {
		return msgstr
	}
	// Fallback to original message
	return msgid
}

// NGettext returns a translation of the provided message using the
// appropriate plural form.
//
// Different languages have different rules for handling plural forms.
// The NGettext method will pick an appropriate form based on the
// integer passed to it.  Normally the translated message will be a
// format string:
//
//     const number = 42
//     fmt.Printf(c.NGettext("%d dog", "%d dogs", number), number)
//
// If no translation is available, one of msgid and msgidPlural will
// be returned, according to the plural rule of Germanic languages
// (i.e. msgid if n==1, and msgidPlural otherwise).
func (c Catalog) NGettext(msgid, msgidPlural string, n uint32) string {
	if msgstr, ok := c.findMsg(msgid, true, n); ok {
		return msgstr
	}
	// Fallback to original message based on Germanic plural rule.
	if n == 1 {
		return msgid
	}
	return msgidPlural
}

// PGettext returns a translation of the provided message using the
// provided context.
//
// In some cases, an single message string may be used in multiple
// places whose translation depends on the context.  This can happen
// with very short message strings, or messages containing homographs.
//
// The PGettext method solves this problem by providing a context
// string together with the message, which is used to look up the
// translation.
//
// If no translation is available, the original message is returned
// without the context.
func (c Catalog) PGettext(msgctxt, msgid string) string {
	if msgstr, ok := c.findMsg(msgctxt+"\x04"+msgid, false, 0); ok {
		return msgstr
	}
	return msgid
}

// NPGettext returns a translation of the provided message using the
// provided context and plural form.
//
// This method combines the functionality of the NGettext and PGettext
// variants.
func (c Catalog) NPGettext(msgctxt, msgid, msgidPlural string, n uint32) string {
	if msgstr, ok := c.findMsg(msgctxt+"\x04"+msgid, true, n); ok {
		return msgstr
	}
	// Fallback to original message based on Germanic plural rule.
	if n == 1 {
		return msgid
	}
	return msgidPlural
}
