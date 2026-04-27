package aliasconfig

import "errors"

// Sentinel errors for alias configuration loading and validation.
// REQ-ALIAS-030 through REQ-ALIAS-037.

// ErrMalformedAliasFile wraps YAML parsing failures.
// REQ-ALIAS-030.
var ErrMalformedAliasFile = errors.New("malformed alias file")

// ErrEmptyAliasEntry indicates an alias entry with blank key or value.
// REQ-ALIAS-031 (empty key), REQ-ALIAS-032 (empty value).
var ErrEmptyAliasEntry = errors.New("empty alias entry")

// ErrInvalidCanonical indicates a canonical value missing the required "provider/model" form.
// REQ-ALIAS-033.
var ErrInvalidCanonical = errors.New("invalid canonical form")

// ErrUnknownProviderInAlias indicates the provider in a canonical value is not registered.
// Only returned in strict validation mode. REQ-ALIAS-034.
var ErrUnknownProviderInAlias = errors.New("unknown provider in alias")

// ErrUnknownModelInAlias indicates the model in a canonical value is not in SuggestedModels.
// Only returned in strict validation mode. REQ-ALIAS-035.
var ErrUnknownModelInAlias = errors.New("unknown model in alias")

// ErrAliasFileTooLarge indicates the file size exceeds the 1 MiB limit.
// REQ-ALIAS-036.
var ErrAliasFileTooLarge = errors.New("alias file too large")
