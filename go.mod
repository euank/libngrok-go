module golang.ngrok.com/ngrok

go 1.20

require (
	github.com/inconshreveable/log15/v3 v3.0.0-testing.5
	github.com/jpillora/backoff v1.0.0
	github.com/stretchr/testify v1.8.0
	go.uber.org/multierr v1.10.0
	golang.ngrok.com/muxado v1.0.0
	golang.org/x/net v0.10.0
	google.golang.org/protobuf v1.28.1
)

// Temporary replace including https://github.com/ngrok/muxado-go/pull/2
// This will hopefully let us see if that PR fixes an issue we see w/ timeouts
replace golang.ngrok.com/muxado => github.com/euank/muxado-go v0.0.0-20230629070835-194c2002f968

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/inconshreveable/log15 v3.0.0-testing.3+incompatible // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
