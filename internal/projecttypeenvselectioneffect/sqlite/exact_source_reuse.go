package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// exactGenesisCanonicalRowExistsTx distinguishes an absent source row from an
// exact immutable row and from an identity collision. The v47/v48 no-replace
// triggers intentionally reject even INSERT OR IGNORE, so callers must prove
// exact reuse before deciding whether an INSERT is expressible.
func exactGenesisCanonicalRowExistsTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	table string,
	refColumn string,
	digestColumn string,
	canonicalColumn string,
	ref string,
	digest string,
	canonical []byte,
) (bool, error) {
	statement := "SELECT " + refColumn + ", " + digestColumn + ", " +
		canonicalColumn +
		" FROM " + table + " WHERE " + refColumn + " = ? OR " +
		digestColumn + " = ?"
	var storedRef string
	var storedDigest string
	var storedCanonical []byte
	err := transaction.ScanOne(
		ctx,
		statement,
		[]any{ref, digest},
		[]any{&storedRef, &storedDigest, &storedCanonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if storedRef != ref ||
		storedDigest != digest ||
		!bytes.Equal(storedCanonical, canonical) {
		return false, fmt.Errorf(
			"%s identity already binds different canonical material",
			table,
		)
	}
	return true, nil
}
