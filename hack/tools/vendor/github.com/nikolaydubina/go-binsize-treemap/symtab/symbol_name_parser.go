package symtab

import (
	"strings"
)

type SymbolName struct {
	PackageParts []string
	SymbolParts  []string
}

func EqSymbolName(a, b SymbolName) bool {
	if len(a.PackageParts) != len(b.PackageParts) {
		return false
	}
	if len(a.SymbolParts) != len(b.SymbolParts) {
		return false
	}
	for i := 0; i < len(a.PackageParts); i++ {
		if a.PackageParts[i] != b.PackageParts[i] {
			return false
		}
	}
	for i := 0; i < len(a.SymbolParts); i++ {
		if a.SymbolParts[i] != b.SymbolParts[i] {
			return false
		}
	}
	return true
}

func ParseSymbolName(r string) SymbolName {
	if strings.HasPrefix(r, "type..eq.struct") {
		return SymbolName{
			PackageParts: []string{"type"},
			SymbolParts:  []string{"", "eq", "struct"},
		}
	}

	// go itab entires can have single comma. treat it as as full path, replace that comma with slash to form path.
	if strings.HasPrefix(r, "go.itab.") {
		numCommas := strings.Count(r, ",")
		if numCommas > 1 {
			// some go itab entries can be complex, with interfaces. ignoring symbols for them.
			// TODO: support go itab interfaces
			r = r[:strings.Index(r, ",")]
		} else {
			r = strings.ReplaceAll(r, ",", "/")
		}
	}

	// pure symbol
	if !strings.ContainsAny(r, "./") {
		return SymbolName{SymbolParts: []string{r}}
	}

	// single-part package just symbol
	if !strings.Contains(r, "/") {
		parts := strings.Split(r, ".")
		return SymbolName{
			PackageParts: parts[:1],
			SymbolParts:  parts[1:],
		}
	}

	// has multi-parts package
	lastSlashIdx := strings.LastIndex(r, "/")

	partsPackage := strings.Split(r[:lastSlashIdx], "/")
	partsSymbol := strings.Split(r[lastSlashIdx:], ".")

	return SymbolName{
		PackageParts: append(partsPackage, partsSymbol[0][1:]),
		SymbolParts:  partsSymbol[1:],
	}
}
