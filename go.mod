module github.com/mphpmaster/word-bomb-tool-go

go 1.24

require (
	github.com/kbinani/screenshot v0.0.0-20240820160931-a8a2c5d0e191
	github.com/lxn/walk v0.0.0-20210112085537-c389da54e794
	github.com/lxn/win v0.0.0-20210218163916-a377121e959e
	golang.org/x/image v0.19.0
	golang.org/x/sys v0.24.0
)

require (
	github.com/gen2brain/shm v0.1.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	gopkg.in/Knetic/govaluate.v3 v3.0.0+incompatible // indirect
)

// proxy.golang.org and the golang.org / gopkg.in vanity hosts are unreachable
// in this build environment, so pull everything straight from GitHub mirrors
// with GOPROXY=direct.
replace (
	golang.org/x/image => github.com/golang/image v0.19.0
	golang.org/x/sys => github.com/golang/sys v0.24.0
	gopkg.in/Knetic/govaluate.v3 => github.com/Knetic/govaluate v3.0.0+incompatible
)
