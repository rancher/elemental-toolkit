/*
Package afis implements Anti-Forensic Information Splitting

The splitter supports secure data destruction crucial for secure on-disk key
management. The key idea is to bloat information and therefor improving the
chance of destroying a single bit of it. The information is bloated in such a
way, that a single missing bit causes the original information become
unrecoverable. The theory behind AFsplitter is presented in TKS1.

The interface is simple. It consists of two functions:

	Split(data, stripes)
	Merge(data, stripes)

Split operates on data and returns information splitted data. Merge does
just the opposite: uses the information stored in data to recover the original
splitted data.


References

AFsplitter reference implementation at http://clemens.endorphin.org/AFsplitter

TKS1 paper at http://clemens.endorphin.org/TKS1-draft.pdf
*/
package afis
