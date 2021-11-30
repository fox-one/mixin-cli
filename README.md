# mixin-cli
Command-line applications to manage mixin dapps

## Install

### From Source Code

```bash
$ git clone git@github.com:fox-one/mixin-cli.git & cd mixin-cli
$ go install
```

## KeyStore

### Format

```json5
{
  "client_id": "",
  "session_id": "",
  "private_key": "",
  "pin_token": "",
  "pin": "", // optional
}
```