module sql

go 1.20

require (
	github.com/eiannone/keyboard v0.0.0-20220611211555-0d226195f203
	github.com/grufgran/config v0.0.0-20230123202511-ef7173f9cfed
	github.com/grufgran/go-terminal v1.0.0
	github.com/grufgran/termMenu v0.0.0-00010101000000-000000000000
)

require golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect

replace (
	github.com/grufgran/config => ../../src/config
	github.com/grufgran/go-terminal => ../../src/go-terminal
	github.com/grufgran/termMenu => ../../src/termMenu
)
