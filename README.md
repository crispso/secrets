# Secrets
A CLI tool for encrytping/decrypting files at Jobbatical.

## Synopsis
```
# To decrypt a file or files.
secrets open [<file path>...] [options]

# To encrypt a file or files.
secrets seal [<file path>...] [options]
```

## Options
```
[--open-all]
[--dry-run]
[--verbose]
[--root <project root>]
[--key <encryption key name>]
```

### Prerequisites
- [Go](https://golang.org/): `secrets` has to be compiled from source.
- [gcloud](https://cloud.google.com/sdk/install): `secrets` uses google cloud kms for crypto.

### Installation process
Clone this repo, build, and install:

```
git clone https://github.com/Jobbatical/secrets
cd secrets
bash build-all
bash install
```
