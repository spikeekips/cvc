# cvc

```
cvc ≈ cobra + viper
```

## Basic Usage

See example/basic.go

```sh
$ go run examples/basic.go -h
naru is API server for SEBAK

Usage:
  naru [flags]

Flags:
  -h, --help                help for naru
      --log-file string     log output file (default "naru.log")
      --log-format string   log format {terminal json} (default "terminal")
      --log-level string    log level {debug error warn crit} (default "debug")
      --set-int int         set integer (default 100)
      --set-string string   set string (default "find me")
      --verbose             verbose
```

```sh
$ go run example/basic.go --log-file /naru.log --log-format json --log-level error --verbose --set-int 3 --set-string 'show me'

# loaded config:
{
  "Verbose": true,
  "SetString": "show me",
  "SetInt": 3,
  "Log": {
    "File": "/naru.log",
    "Level": "error",
    "Format": "json"
  }
}
```
