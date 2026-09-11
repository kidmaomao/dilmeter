module gitlab.com/prilus/mabidilmeter

go 1.24.0

toolchain go1.24.11

require (
	github.com/gopacket/gopacket v1.5.0
	github.com/pirogom/walk v0.0.0-20240303053834-84deab9e0f34
	github.com/pirogom/walkmgr v0.0.0-20240325050432-be0ce8009c90
	golang.org/x/net v0.50.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/jchv/go-webview2 v0.0.0-20260205173254-56598839c808 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/pirogom/win v0.0.0-20220414140423-11dc73c0bb71 // indirect
	golang.org/x/sys v0.41.0 // indirect
	gopkg.in/Knetic/govaluate.v3 v3.0.0 // indirect
)

replace github.com/jchv/go-webview2 => ./webview2-local
