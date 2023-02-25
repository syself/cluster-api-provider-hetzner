package symtab

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// GoSymtabParser parser symtab files produced by `go tool nm`.
// https://pkg.go.dev/cmd/nm
type GoSymtabParser struct{}

func (s GoSymtabParser) ParseSymtab(lines []string) (*SymtabFile, error) {
	var f SymtabFile

	f.Entries = make([]SymtabEntry, 0, len(lines))

	for i, line := range lines {
		e, err := parseGoSymtabLine(line)
		if err != nil {
			err := fmt.Errorf("error parasing symtab at line num(%d): %w: line: %s", i, err, line)
			log.Println(err.Error())
			continue
		}
		f.Entries = append(f.Entries, e)
	}

	return &f, nil
}

// Utilizing observation that:
// A) symbol type has to be always present
// B) symbol size is right most before symbol type
// D) symbol address is optional
// E) symbol name is optional and everything to the right
func parseGoSymtabLine(line string) (SymtabEntry, error) {
	fields := strings.Fields(line)

	if len(fields) < 2 {
		return SymtabEntry{}, fmt.Errorf("at least two entires required in row")
	}

	// find index of type
	idxType := -1
	for i := 0; i < len(fields); i++ {
		if _, ok := SymbolTypes[SymbolType(fields[i])]; ok {
			idxType = i
			break
		}
	}
	if idxType == -1 {
		return SymtabEntry{}, fmt.Errorf("symbol type is not found in row")
	}
	if !(idxType == 1 || idxType == 2) {
		return SymtabEntry{}, fmt.Errorf("expected symbol type be either 2nd or 3rd element in row found at(%d)", idxType)
	}

	size, err := strconv.Atoi(fields[idxType-1])
	if err != nil {
		return SymtabEntry{}, fmt.Errorf("wrong size field: %w", err)
	}

	entry := SymtabEntry{
		Size: uint(size),
		Type: SymbolType(fields[idxType]),
	}

	if idxType == 2 {
		entry.Address = fields[idxType-2]
	}

	if idxType < len(fields)-1 {
		entry.SymbolName = strings.Join(fields[idxType+1:], " ")

	}

	return entry, nil
}
