package symtab

// https://pkg.go.dev/cmd/nm
type SymbolType string

const (
	Text             SymbolType = "T" // T	text (code) segment symbol
	StaticText       SymbolType = "t" // t	static text segment symbol
	ReadOnly         SymbolType = "R" // R	read-only data segment symbol
	StaticReadOnly   SymbolType = "r" // r	static read-only data segment symbol
	Data             SymbolType = "D" // D	data segment symbol
	StaticData       SymbolType = "d" // d	static data segment symbol
	BSSSegment       SymbolType = "B" // B	bss segment symbol
	StaticBSSSegment SymbolType = "b" // b	static bss segment symbol
	Constant         SymbolType = "C" // C	constant address
	Undefined        SymbolType = "U" // U	referenced but undefined symbol

	// special observed
	Underscore SymbolType = "_" // cpp?
)

// SymbolTypes has all recognized symbol types.
var SymbolTypes map[SymbolType]bool = map[SymbolType]bool{
	Text:             true,
	StaticText:       true,
	ReadOnly:         true,
	StaticReadOnly:   true,
	Data:             true,
	StaticData:       true,
	BSSSegment:       true,
	StaticBSSSegment: true,
	Constant:         true,
	Undefined:        true,
	Underscore:       true,
}

// SymtabEntry single symbol details from symtab Go file.
type SymtabEntry struct {
	Address    string // hex
	Type       SymbolType
	SymbolName string // might require demangling, for example for c++
	Size       uint   // in bytes
}

// SymtabFile stores details from single Go symtab file produced from analyzing Go executable.
type SymtabFile struct {
	Entries []SymtabEntry
}
