package types

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"unicode"

	"github.com/mudler/topsort"
	"github.com/philopon/go-toposort"
	"github.com/pkg/errors"
)

// PackageAssert represent a package assertion.
// It is composed of a Package and a Value which is indicating the absence or not
// of the associated package state.
type PackageAssert struct {
	Package *Package
	Value   bool
	Hash    PackageHash
}

func (a *PackageAssert) String() string {
	var msg string
	if a.Value {
		msg = "installed"
	} else {
		msg = "not installed"
	}
	return fmt.Sprintf("%s/%s %s %s", a.Package.GetCategory(), a.Package.GetName(), a.Package.GetVersion(), msg)
}

func (assertions PackagesAssertions) EnsureOrder() PackagesAssertions {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}
	fingerprints := []string{}

	tmpMap := map[string]PackageAssert{}

	for _, a := range assertions {
		tmpMap[a.Package.GetFingerPrint()] = a
		fingerprints = append(fingerprints, a.Package.GetFingerPrint())
		unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered

		if a.Value {
			unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered
		} else {
			orderedAssertions = append(orderedAssertions, a) // Keep last the ones which are not meant to be installed
		}
	}

	sort.Sort(unorderedAssertions)

	// Build a topological graph
	graph := toposort.NewGraph(len(unorderedAssertions))
	graph.AddNodes(fingerprints...)
	for _, a := range unorderedAssertions {
		for _, req := range a.Package.GetRequires() {
			graph.AddEdge(a.Package.GetFingerPrint(), req.GetFingerPrint())
		}
	}
	result, ok := graph.Toposort()
	if !ok {
		panic("Cycle found")
	}
	for _, res := range result {
		a, ok := tmpMap[res]
		if !ok {
			panic("fail")
			//	continue
		}
		orderedAssertions = append(orderedAssertions, a)
		//	orderedAssertions = append(PackagesAssertions{a}, orderedAssertions...) // push upfront
	}
	//helpers.ReverseAny(orderedAssertions)
	return orderedAssertions
}

// SearchByName searches a string matching a package in the assetion list
// in the category/name format
func (assertions PackagesAssertions) SearchByName(f string) *PackageAssert {
	for _, a := range assertions {
		if a.Value {
			if a.Package.GetPackageName() == f {
				return &a
			}
		}
	}

	return nil
}
func (assertions PackagesAssertions) Search(f string) *PackageAssert {
	for _, a := range assertions {
		if a.Value {
			if a.Package.GetFingerPrint() == f {
				return &a
			}
		}
	}

	return nil
}

func (assertions PackagesAssertions) Order(definitiondb PackageDatabase, fingerprint string) (PackagesAssertions, error) {

	orderedAssertions := PackagesAssertions{}
	unorderedAssertions := PackagesAssertions{}

	tmpMap := map[string]PackageAssert{}
	graph := topsort.NewGraph()
	for _, a := range assertions {
		graph.AddNode(a.Package.GetFingerPrint())
		tmpMap[a.Package.GetFingerPrint()] = a
		unorderedAssertions = append(unorderedAssertions, a) // Build a list of the ones that must be ordered
	}

	sort.Sort(unorderedAssertions)
	// Build a topological graph
	for _, a := range unorderedAssertions {
		currentPkg := a.Package
		added := map[string]interface{}{}
	REQUIRES:
		for _, requiredDef := range currentPkg.GetRequires() {
			if def, err := definitiondb.FindPackage(requiredDef); err == nil { // Provides: Get a chance of being override here
				requiredDef = def
			}

			// We cannot search for fingerprint, as we could have selector in versions.
			// We know that the assertions are unique for packages, so look for a package with such name in the assertions
			req := assertions.SearchByName(requiredDef.GetPackageName())
			if req != nil {
				requiredDef = req.Package
			}
			if _, ok := added[requiredDef.GetFingerPrint()]; ok {
				continue REQUIRES
			}
			// Expand also here, as we need to order them (or instead the solver should give back the dep correctly?)
			graph.AddEdge(currentPkg.GetFingerPrint(), requiredDef.GetFingerPrint())
			added[requiredDef.GetFingerPrint()] = true
		}
	}
	result, err := graph.TopSort(fingerprint)
	if err != nil {
		return nil, errors.Wrap(err, "fail on sorting "+fingerprint)
	}
	for _, res := range result {
		a, ok := tmpMap[res]
		if !ok {
			//return nil, errors.New("fail looking for " + res)
			// Since now we don't return the entire world as part of assertions
			// if we don't find any reference must be because fingerprint we are analyzing (which is the one we are ordering against)
			// is not part of the assertions, thus we can omit it from the result
			continue
		}
		orderedAssertions = append(orderedAssertions, a)
		//	orderedAssertions = append(PackagesAssertions{a}, orderedAssertions...) // push upfront
	}
	//helpers.ReverseAny(orderedAssertions)
	return orderedAssertions, nil
}

func (a PackagesAssertions) Len() int      { return len(a) }
func (a PackagesAssertions) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PackagesAssertions) Less(i, j int) bool {

	iRunes := []rune(a[i].Package.GetName())
	jRunes := []rune(a[j].Package.GetName())

	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}

	return false

}

// TrueLen returns the lenth of true assertions in the slice
func (assertions PackagesAssertions) TrueLen() int {
	count := 0
	for _, ass := range assertions {
		if ass.Value {
			count++
		}
	}

	return count
}

// HashFrom computes the assertion hash From a given package. It drops it from the assertions
// and checks it's not the only one. if it's unique it marks it specially - so the hash
// which is generated is unique for the selected package
func (assertions PackagesAssertions) HashFrom(p *Package) string {
	return assertions.SaltedHashFrom(p, map[string]string{})
}

func (assertions PackagesAssertions) AssertionHash() string {
	return assertions.SaltedAssertionHash(map[string]string{})
}

func (assertions PackagesAssertions) SaltedHashFrom(p *Package, salts map[string]string) string {
	var assertionhash string

	// When we don't have any solution to hash for, we need to generate an UUID by ourselves
	latestsolution := assertions.Drop(p)
	if latestsolution.TrueLen() == 0 {
		// Preserve the hash if supplied of marked packages
		marked := p.Mark()
		if markedHash, exists := salts[p.GetFingerPrint()]; exists {
			salts[marked.GetFingerPrint()] = markedHash
		}
		assertionhash = assertions.Mark(p).SaltedAssertionHash(salts)
	} else {
		assertionhash = latestsolution.SaltedAssertionHash(salts)
	}
	return assertionhash
}

func (assertions PackagesAssertions) SaltedAssertionHash(salts map[string]string) string {
	var fingerprint string
	for _, assertion := range assertions { // Note: Always order them first!
		if assertion.Value { // Tke into account only dependencies installed (get fingerprint of subgraph)
			salt, exists := salts[assertion.Package.GetFingerPrint()]
			if exists {
				fingerprint += assertion.String() + salt + "\n"

			} else {
				fingerprint += assertion.String() + "\n"
			}
		}
	}
	hash := sha256.Sum256([]byte(fingerprint))
	return fmt.Sprintf("%x", hash)
}

func (assertions PackagesAssertions) Drop(p *Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if !a.Package.Matches(p) {
			ass = append(ass, a)
		}
	}
	return ass
}

// Cut returns an assertion list of installed (filter by Value) "cutted" until the package is found (included)
func (assertions PackagesAssertions) Cut(p *Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if a.Value {
			ass = append(ass, a)
			if a.Package.Matches(p) {
				break
			}
		}
	}
	return ass
}

// Mark returns a new assertion with the package marked
func (assertions PackagesAssertions) Mark(p *Package) PackagesAssertions {
	ass := PackagesAssertions{}

	for _, a := range assertions {
		if a.Package.Matches(p) {
			marked := a.Package.Mark()
			a = PackageAssert{Package: marked, Value: a.Value, Hash: a.Hash}
		}
		ass = append(ass, a)
	}
	return ass
}
