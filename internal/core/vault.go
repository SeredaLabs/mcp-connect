package core

import "errors"

var ErrSecretMissing = errors.New("Credentials not found in the system secret store")

type OSVault struct{}
