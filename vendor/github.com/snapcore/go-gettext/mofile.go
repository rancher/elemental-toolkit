package gettext

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/snapcore/go-gettext/pluralforms"
)

const le_magic = 0x950412de
const be_magic = 0xde120495

type header struct {
	Magic          uint32
	Version        uint32
	NumStrings     uint32
	OrigTabOffset  uint32
	TransTabOffset uint32
	HashTabSize    uint32
	HashTabOffset  uint32
}

func (header header) get_major_version() uint32 {
	return header.Version >> 16
}

func (header header) get_minor_version() uint32 {
	return header.Version & 0xffff
}

type mocatalog struct {
	m     *fileMapping
	order binary.ByteOrder

	numStrings int
	origTab    []byte
	transTab   []byte
	hashTab    []byte

	info        map[string]string
	language    string
	pluralforms pluralforms.Expression
	charset     string
}

func (catalog *mocatalog) findMsg(msgid string, usePlural bool, n uint32) (msgstr string, ok bool) {
	idx, ok := catalog.msgIndex(msgid)
	if !ok {
		return "", false
	}
	var plural int
	if usePlural {
		if catalog.pluralforms != nil {
			plural = catalog.pluralforms.Eval(n)
		} else {
			// Bogus/missing pluralforms in mo: Use the Germanic
			// plural rule.
			if n == 1 {
				plural = 0
			} else {
				plural = 1
			}
		}
	}
	return string(catalog.msgStr(idx, plural)), true
}

func (catalog *mocatalog) msgID(idx int) []byte {
	strLen := catalog.order.Uint32(catalog.origTab[8*idx:])
	strOffset := catalog.order.Uint32(catalog.origTab[8*idx+4:])
	msgid := catalog.m.data[strOffset : strOffset+strLen]

	zero := bytes.IndexByte(msgid, '\x00')
	if zero >= 0 {
		msgid = msgid[:zero]
	}
	return msgid
}

func (catalog *mocatalog) msgStr(idx, n int) []byte {
	strLen := catalog.order.Uint32(catalog.transTab[8*idx:])
	strOffset := catalog.order.Uint32(catalog.transTab[8*idx+4:])
	msgstr := catalog.m.data[strOffset : strOffset+strLen]

	for ; n >= 0; n-- {
		zero := bytes.IndexByte(msgstr, '\x00')
		if n == 0 {
			if zero >= 0 {
				msgstr = msgstr[:zero]
			}
			break
		} else {
			// fast forward to next string.  If there is
			// no nul byte, then this is a no-op
			msgstr = msgstr[zero+1:]
		}
	}
	return msgstr
}

// hashString implements libintl's hash_string() algorithm
func hashString(s string) uint32 {
	const hashWordBits = 32
	var hval, g uint32

	for i := 0; i < len(s); i++ {
		hval <<= 4
		hval += uint32(s[i])
		g = hval & (0xf << (hashWordBits - 4))
		if g != 0 {
			hval ^= g >> (hashWordBits - 8)
			hval ^= g
		}
	}
	return hval
}

func (catalog *mocatalog) msgIndex(msgid string) (idx int, ok bool) {
	// Use the hash table if available
	if catalog.hashTab != nil {
		// Hash table lookup adapted from libintl's _nl_find_msg()
		hval := hashString(msgid)
		hashSize := uint32(len(catalog.hashTab) / 4)
		idx := hval % hashSize
		incr := 1 + (hval % (hashSize - 2))

		for {
			nstr := catalog.order.Uint32(catalog.hashTab[4*idx:])
			if nstr == 0 {
				// Hash table entry is empty
				return 0, false
			}

			nstr -= 1
			if string(catalog.msgID(int(nstr))) == msgid {
				return int(nstr), true
			}
			if idx >= hashSize-incr {
				idx -= hashSize - incr
			} else {
				idx += incr
			}
		}
	}

	// Fall back to a binary search over origTab message IDs
	idx = sort.Search(catalog.numStrings, func(i int) bool {
		return string(catalog.msgID(i)) >= msgid
	})
	if idx < catalog.numStrings && string(catalog.msgID(idx)) == msgid {
		return idx, true
	}
	return 0, false
}

func (catalog *mocatalog) read_info(info string) error {
	catalog.info = make(map[string]string)
	lastk := ""
	for _, line := range strings.Split(info, "\n") {
		item := strings.TrimSpace(line)
		if len(item) == 0 {
			continue
		}
		var k string
		var v string
		if strings.Contains(item, ":") {
			tmp := strings.SplitN(item, ":", 2)
			k = strings.ToLower(strings.TrimSpace(tmp[0]))
			v = strings.TrimSpace(tmp[1])
			catalog.info[k] = v
			lastk = k
		} else if len(lastk) != 0 {
			catalog.info[lastk] += "\n" + item
		}
		if k == "content-type" {
			catalog.charset = strings.Split(v, "charset=")[1]
		} else if k == "plural-forms" {
			p := strings.Split(v, ";")[1]
			s := strings.Split(p, "plural=")[1]
			expr, err := pluralforms.Compile(s)
			if err != nil {
				return err
			}
			catalog.pluralforms = expr
		}
	}
	return nil
}

func validateStringTable(m *fileMapping, table []byte, numStrings int, order binary.ByteOrder) error {
	for i := 0; i < numStrings; i++ {
		strLen := order.Uint32(table[8*i:])
		strOffset := order.Uint32(table[8*i+4:])
		if int(strLen+strOffset) > len(m.data) {
			return fmt.Errorf("string %d data (len=%x, offset=%x) is out of bounds", i, strLen, strOffset)
		}
	}
	return nil
}

func validateHashTable(table []byte, numStrings int, order binary.ByteOrder) error {
	for i := 0; i < numStrings; i++ {
		strIndex := order.Uint32(table[4*i:])
		// hash entries are either zero or a string index
		// incremented by one
		if int(strIndex) >= numStrings+1 {
			return fmt.Errorf("hash table is corrupt")
		}
	}
	return nil
}

// ParseMO parses a mo file into a Catalog if possible.
func ParseMO(file *os.File) (Catalog, error) {
	mo, err := parseMO(file)
	if err != nil {
		return Catalog{}, err
	}
	return Catalog{[]*mocatalog{mo}}, nil
}

func parseMO(file *os.File) (*mocatalog, error) {
	m, err := openMapping(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		if m != nil {
			m.Close()
		}
	}()

	var header header
	headerSize := binary.Size(&header)
	if len(m.data) < headerSize {
		return nil, fmt.Errorf("message catalogue is too short")
	}

	var order binary.ByteOrder = binary.LittleEndian
	magic := order.Uint32(m.data)
	switch magic {
	case le_magic:
		// nothing
	case be_magic:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("Wrong magic: %d", magic)
	}
	if err := binary.Read(bytes.NewBuffer(m.data[:headerSize]), order, &header); err != nil {
		return nil, err
	}
	if header.get_major_version() != 0 && header.get_major_version() != 1 {
		return nil, fmt.Errorf("Unsupported version: %d.%d", header.get_major_version(), header.get_minor_version())
	}
	if int64(int(header.NumStrings)) != int64(header.NumStrings) {
		return nil, fmt.Errorf("too many strings in catalog")
	}
	numStrings := int(header.NumStrings)

	if int(header.OrigTabOffset+8*header.NumStrings) > len(m.data) {
		return nil, fmt.Errorf("original strings table out of bounds")
	}
	origTab := m.data[header.OrigTabOffset : header.OrigTabOffset+8*header.NumStrings]
	if err := validateStringTable(m, origTab, numStrings, order); err != nil {
		return nil, err
	}

	if int(header.TransTabOffset+8*header.NumStrings) > len(m.data) {
		return nil, fmt.Errorf("translated strings table out of bounds")
	}
	transTab := m.data[header.TransTabOffset : header.TransTabOffset+8*header.NumStrings]
	if err := validateStringTable(m, transTab, numStrings, order); err != nil {
		return nil, err
	}

	var hashTab []byte
	if header.HashTabSize > 2 {
		if int(header.HashTabOffset+4*header.HashTabSize) > len(m.data) {
			return nil, fmt.Errorf("hash table out of bounds")
		}
		hashTab = m.data[header.HashTabOffset : header.HashTabOffset+4*header.HashTabSize]
		if err := validateHashTable(hashTab, numStrings, order); err != nil {
			return nil, err
		}
	}

	catalog := &mocatalog{
		m:     m,
		order: order,

		numStrings: numStrings,
		origTab:    origTab,
		transTab:   transTab,
		hashTab:    hashTab,
	}
	// Read catalog header if available
	if catalog.numStrings > 0 && len(catalog.msgID(0)) == 0 {
		if err := catalog.read_info(string(catalog.msgStr(0, 0))); err != nil {
			return nil, err
		}
	}

	m = nil
	return catalog, nil
}
