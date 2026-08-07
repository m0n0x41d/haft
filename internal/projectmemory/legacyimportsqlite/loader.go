// Package legacyimportsqlite observes the legacy SQLite project-memory rows
// without mutating them. It maps exact SQLite values into the effect-free
// legacyimport carrier/observation algebra; it does not infer typed semantics,
// select a ProjectTypeEnv, or expose an import-apply capability.
package legacyimportsqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	CoreSnapshotClassifierVersionV1 = "haft.legacy-sqlite-core-classifier/v1"
	legacyRowSchemaVersionV1        = "haft.legacy-sqlite-row/v1"
)

var ErrUnsupportedLegacySchema = errors.New("unsupported legacy SQLite schema")

type sourceKind string

const (
	sourceObject      sourceKind = "object"
	sourceAssociation sourceKind = "association"
)

type tableDescriptor struct {
	name       string
	kind       sourceKind
	keyColumns []string
	source     string
	target     string
	label      string
}

var coreTableDescriptors = []tableDescriptor{
	{
		name:       "artifacts",
		kind:       sourceObject,
		keyColumns: []string{"id"},
	},
	{
		name:       "artifact_links",
		kind:       sourceAssociation,
		keyColumns: []string{"source_id", "target_id", "link_type"},
		source:     "source_id",
		target:     "target_id",
		label:      "link_type",
	},
	{
		name:       "holons",
		kind:       sourceObject,
		keyColumns: []string{"id"},
	},
	{
		name:       "relations",
		kind:       sourceAssociation,
		keyColumns: []string{"source_id", "target_id", "relation_type"},
		source:     "source_id",
		target:     "target_id",
		label:      "relation_type",
	},
}

type CoreSnapshotLoader struct {
	database *sql.DB
}

func NewCoreSnapshotLoader(database *sql.DB) (CoreSnapshotLoader, error) {
	if database == nil {
		return CoreSnapshotLoader{}, fmt.Errorf("legacy SQLite database is required")
	}
	return CoreSnapshotLoader{database: database}, nil
}

func (loader CoreSnapshotLoader) Load(
	ctx context.Context,
) (legacyimport.LegacySourceSnapshot, error) {
	transaction, err := loader.database.BeginTx(
		ctx,
		&sql.TxOptions{ReadOnly: true},
	)
	if err != nil {
		return legacyimport.LegacySourceSnapshot{}, fmt.Errorf(
			"begin legacy SQLite snapshot: %w",
			err,
		)
	}
	defer transaction.Rollback()

	snapshots := make([]legacyimport.CarrierSnapshot, 0)
	observations := make([]legacyimport.SubjectObservation, 0)
	for _, descriptor := range coreTableDescriptors {
		tableSnapshots, tableObservations, loadErr := loadTable(
			ctx,
			transaction,
			descriptor,
		)
		if loadErr != nil {
			return legacyimport.LegacySourceSnapshot{}, loadErr
		}
		snapshots = append(snapshots, tableSnapshots...)
		observations = append(observations, tableObservations...)
	}

	catalog, err := legacyimport.NewCarrierCatalog(snapshots)
	if err != nil {
		return legacyimport.LegacySourceSnapshot{}, fmt.Errorf(
			"build legacy SQLite carrier catalog: %w",
			err,
		)
	}
	observationSet, err := legacyimport.NewObservationSet(observations)
	if err != nil {
		return legacyimport.LegacySourceSnapshot{}, fmt.Errorf(
			"build legacy SQLite observation set: %w",
			err,
		)
	}
	source, err := legacyimport.NewLegacySourceSnapshot(
		catalog,
		observationSet,
	)
	if err != nil {
		return legacyimport.LegacySourceSnapshot{}, fmt.Errorf(
			"build legacy SQLite source snapshot: %w",
			err,
		)
	}
	if err := transaction.Commit(); err != nil {
		return legacyimport.LegacySourceSnapshot{}, fmt.Errorf(
			"finish legacy SQLite read snapshot: %w",
			err,
		)
	}
	return source, nil
}

func (loader CoreSnapshotLoader) DryRun(
	ctx context.Context,
	projectID projectidentity.ProjectID,
) (legacyimport.DryRunReport, error) {
	source, err := loader.Load(ctx)
	if err != nil {
		return legacyimport.DryRunReport{}, err
	}
	classifications, err := conservativeClassifications(source.Observations())
	if err != nil {
		return legacyimport.DryRunReport{}, err
	}
	classifier, err := legacyimport.NewClassifierVersion(
		CoreSnapshotClassifierVersionV1,
	)
	if err != nil {
		return legacyimport.DryRunReport{}, fmt.Errorf(
			"construct legacy SQLite classifier version: %w",
			err,
		)
	}
	report, err := legacyimport.NewDryRunReport(
		projectID,
		classifier,
		source,
		classifications,
	)
	if err != nil {
		return legacyimport.DryRunReport{}, fmt.Errorf(
			"build legacy SQLite dry-run report: %w",
			err,
		)
	}
	return report, nil
}

type tableColumn struct {
	name         string
	declaredType string
}

type rowValue struct {
	storageClass string
	bytes        []byte
}

type observedRow struct {
	table  string
	values map[string]rowValue
	bytes  []byte
	digest string
}

func loadTable(
	ctx context.Context,
	transaction *sql.Tx,
	descriptor tableDescriptor,
) (
	[]legacyimport.CarrierSnapshot,
	[]legacyimport.SubjectObservation,
	error,
) {
	columns, err := loadTableColumns(ctx, transaction, descriptor)
	if err != nil {
		return nil, nil, err
	}
	rows, err := loadRows(ctx, transaction, descriptor, columns)
	if err != nil {
		return nil, nil, err
	}
	snapshots := make([]legacyimport.CarrierSnapshot, 0, len(rows))
	observations := make([]legacyimport.SubjectObservation, 0, len(rows))
	for _, row := range rows {
		snapshot, observation, mapErr := mapObservedRow(descriptor, row)
		if mapErr != nil {
			return nil, nil, mapErr
		}
		snapshots = append(snapshots, snapshot)
		observations = append(observations, observation)
	}
	return snapshots, observations, nil
}

func loadTableColumns(
	ctx context.Context,
	transaction *sql.Tx,
	descriptor tableDescriptor,
) ([]tableColumn, error) {
	const query = `
		SELECT cid, name, type, "notnull", dflt_value, pk
		FROM pragma_table_info(?)
		ORDER BY cid`
	rows, err := transaction.QueryContext(ctx, query, descriptor.name)
	if err != nil {
		return nil, fmt.Errorf(
			"inspect legacy SQLite table %s: %w",
			descriptor.name,
			err,
		)
	}
	defer rows.Close()

	columns := make([]tableColumn, 0)
	for rows.Next() {
		var (
			index        int
			name         string
			declaredType string
			notNull      int
			defaultValue any
			primaryKey   int
		)
		scanErr := rows.Scan(
			&index,
			&name,
			&declaredType,
			&notNull,
			&defaultValue,
			&primaryKey,
		)
		if scanErr != nil {
			return nil, fmt.Errorf(
				"scan legacy SQLite table %s column: %w",
				descriptor.name,
				scanErr,
			)
		}
		columns = append(columns, tableColumn{
			name:         name,
			declaredType: declaredType,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate legacy SQLite table %s columns: %w",
			descriptor.name,
			err,
		)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf(
			"%w: required table %s is absent",
			ErrUnsupportedLegacySchema,
			descriptor.name,
		)
	}
	if err := requireColumns(descriptor, columns); err != nil {
		return nil, err
	}
	sort.Slice(columns, func(left, right int) bool {
		return columns[left].name < columns[right].name
	})
	return columns, nil
}

func requireColumns(
	descriptor tableDescriptor,
	columns []tableColumn,
) error {
	available := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		available[column.name] = struct{}{}
	}
	required := append([]string(nil), descriptor.keyColumns...)
	if descriptor.kind == sourceAssociation {
		required = append(
			required,
			descriptor.source,
			descriptor.target,
			descriptor.label,
		)
	}
	for _, name := range required {
		if _, found := available[name]; found {
			continue
		}
		return fmt.Errorf(
			"%w: table %s lacks required column %s",
			ErrUnsupportedLegacySchema,
			descriptor.name,
			name,
		)
	}
	return nil
}

func loadRows(
	ctx context.Context,
	transaction *sql.Tx,
	descriptor tableDescriptor,
	columns []tableColumn,
) ([]observedRow, error) {
	query := buildValueQuery(descriptor.name, columns)
	rows, err := transaction.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"read legacy SQLite table %s: %w",
			descriptor.name,
			err,
		)
	}
	defer rows.Close()

	result := make([]observedRow, 0)
	for rows.Next() {
		row, scanErr := scanObservedRow(rows, descriptor.name, columns)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate legacy SQLite table %s: %w",
			descriptor.name,
			err,
		)
	}
	sort.Slice(result, func(left, right int) bool {
		return string(result[left].bytes) < string(result[right].bytes)
	})
	return result, nil
}

func buildValueQuery(table string, columns []tableColumn) string {
	parts := make([]string, 0, len(columns)*2)
	for _, column := range columns {
		quoted := quoteIdentifier(column.name)
		parts = append(parts, "typeof("+quoted+")")
		parts = append(parts, "CAST("+quoted+" AS BLOB)")
	}
	return "SELECT " + strings.Join(parts, ", ") +
		" FROM " + quoteIdentifier(table)
}

func scanObservedRow(
	rows *sql.Rows,
	table string,
	columns []tableColumn,
) (observedRow, error) {
	values := make([]any, len(columns)*2)
	destinations := make([]any, len(values))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := rows.Scan(destinations...); err != nil {
		return observedRow{}, fmt.Errorf(
			"scan legacy SQLite table %s row: %w",
			table,
			err,
		)
	}

	observed := make(map[string]rowValue, len(columns))
	canonical := make([]canonicalColumn, 0, len(columns))
	for index, column := range columns {
		value, err := decodeRowValue(
			table,
			column.name,
			values[index*2],
			values[index*2+1],
		)
		if err != nil {
			return observedRow{}, err
		}
		observed[column.name] = value
		canonical = append(canonical, canonicalColumn{
			Name:         column.name,
			DeclaredType: column.declaredType,
			StorageClass: value.storageClass,
			Base64Bytes:  base64.RawURLEncoding.EncodeToString(value.bytes),
		})
	}
	envelope := canonicalRow{
		SchemaVersion: legacyRowSchemaVersionV1,
		Table:         table,
		Columns:       canonical,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return observedRow{}, fmt.Errorf(
			"encode legacy SQLite table %s row: %w",
			table,
			err,
		)
	}
	sum := sha256.Sum256(encoded)
	return observedRow{
		table:  table,
		values: observed,
		bytes:  encoded,
		digest: hex.EncodeToString(sum[:]),
	}, nil
}

func decodeRowValue(
	table string,
	column string,
	storageClassRaw any,
	valueRaw any,
) (rowValue, error) {
	storageClass, ok := storageClassRaw.(string)
	if !ok {
		return rowValue{}, fmt.Errorf(
			"legacy SQLite table %s column %s has non-text storage class",
			table,
			column,
		)
	}
	if storageClass == "null" {
		return rowValue{storageClass: storageClass}, nil
	}
	bytes, ok := valueRaw.([]byte)
	if !ok {
		text, textOK := valueRaw.(string)
		if !textOK {
			return rowValue{}, fmt.Errorf(
				"legacy SQLite table %s column %s cannot be read as bytes",
				table,
				column,
			)
		}
		bytes = []byte(text)
	}
	return rowValue{
		storageClass: storageClass,
		bytes:        append([]byte(nil), bytes...),
	}, nil
}

type canonicalRow struct {
	SchemaVersion string            `json:"schema_version"`
	Table         string            `json:"table"`
	Columns       []canonicalColumn `json:"columns"`
}

type canonicalColumn struct {
	Name         string `json:"name"`
	DeclaredType string `json:"declared_type"`
	StorageClass string `json:"storage_class"`
	Base64Bytes  string `json:"base64_bytes"`
}

func mapObservedRow(
	descriptor tableDescriptor,
	row observedRow,
) (
	legacyimport.CarrierSnapshot,
	legacyimport.SubjectObservation,
	error,
) {
	key, err := encodedKey(row, descriptor.keyColumns)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	ref, err := typedmemory.NewCarrierRef(
		"legacy-sqlite-row:" + descriptor.name + ":" + key,
	)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	edition, err := typedmemory.NewCarrierEdition(
		"legacy-sqlite-row-edition:" + row.digest,
	)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	coordinate, err := legacyimport.NewSourceCoordinate(
		"sqlite/" + descriptor.name + "/" + key,
	)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	format, err := legacyimport.NewCarrierFormat(
		"application/vnd.haft.legacy-sqlite-row+json;table=" +
			descriptor.name +
			";version=1",
	)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	identity, err := carrierLegacyIdentity(descriptor, row)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	snapshot, err := legacyimport.NewCarrierSnapshot(
		coordinate,
		ref,
		edition,
		format,
		row.bytes,
		identity,
	)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	observation, err := rowObservation(descriptor, row, snapshot)
	if err != nil {
		return legacyimport.CarrierSnapshot{}, nil, err
	}
	return snapshot, observation, nil
}

func carrierLegacyIdentity(
	descriptor tableDescriptor,
	row observedRow,
) (legacyimport.CarrierLegacyIdentity, error) {
	if descriptor.kind == sourceAssociation {
		return legacyimport.NoLegacyIdentity{}, nil
	}
	raw, err := exactTextValue(row, descriptor.keyColumns[0])
	if err != nil {
		return nil, err
	}
	ref, err := legacyObjectIdentity(raw)
	if err != nil {
		return nil, err
	}
	return legacyimport.NewIdentifiedLegacyCarrier(ref)
}

func rowObservation(
	descriptor tableDescriptor,
	row observedRow,
	snapshot legacyimport.CarrierSnapshot,
) (legacyimport.SubjectObservation, error) {
	if descriptor.kind == sourceObject {
		return objectObservation(descriptor, row, snapshot)
	}
	return associationObservation(descriptor, row, snapshot)
}

func objectObservation(
	descriptor tableDescriptor,
	row observedRow,
	snapshot legacyimport.CarrierSnapshot,
) (legacyimport.SubjectObservation, error) {
	raw, err := exactTextValue(row, descriptor.keyColumns[0])
	if err != nil {
		return nil, err
	}
	subject, err := legacyimport.NewSemanticSubjectRef(
		"legacy-object:" + encodeOpaque(raw),
	)
	if err != nil {
		return nil, err
	}
	return legacyimport.NewCarrierObservation(subject, snapshot)
}

func associationObservation(
	descriptor tableDescriptor,
	row observedRow,
	snapshot legacyimport.CarrierSnapshot,
) (legacyimport.SubjectObservation, error) {
	sourceRaw, err := exactTextValue(row, descriptor.source)
	if err != nil {
		return nil, err
	}
	targetRaw, err := exactTextValue(row, descriptor.target)
	if err != nil {
		return nil, err
	}
	labelRaw, err := exactTextValue(row, descriptor.label)
	if err != nil {
		return nil, err
	}
	subject, err := legacyimport.NewSemanticSubjectRef(
		"legacy-association:" + descriptor.name + ":" + row.digest,
	)
	if err != nil {
		return nil, err
	}
	source, err := legacyObjectIdentity(sourceRaw)
	if err != nil {
		return nil, err
	}
	target, err := legacyObjectIdentity(targetRaw)
	if err != nil {
		return nil, err
	}
	label, err := legacyimport.NewAssociationLabel(
		"legacy-label:" + descriptor.name + ":" + encodeOpaque(labelRaw),
	)
	if err != nil {
		return nil, err
	}
	return legacyimport.NewAssociationObservation(
		subject,
		snapshot,
		source,
		target,
		label,
	)
}

func conservativeClassifications(
	set legacyimport.ObservationSet,
) ([]legacyimport.SubjectClassification, error) {
	grouped := make(map[string][]legacyimport.SubjectObservation)
	for _, observation := range set.Values() {
		subject := observation.Subject().String()
		grouped[subject] = append(grouped[subject], observation)
	}
	subjects := make([]string, 0, len(grouped))
	for subject := range grouped {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)

	result := make([]legacyimport.SubjectClassification, 0, len(subjects))
	for _, subjectRaw := range subjects {
		classification, err := classifyConservatively(
			subjectRaw,
			grouped[subjectRaw],
		)
		if err != nil {
			return nil, err
		}
		result = append(result, classification)
	}
	return result, nil
}

func classifyConservatively(
	subjectRaw string,
	observations []legacyimport.SubjectObservation,
) (legacyimport.SubjectClassification, error) {
	subject, err := legacyimport.NewSemanticSubjectRef(subjectRaw)
	if err != nil {
		return nil, err
	}
	carriers := make([]legacyimport.CarrierObservation, 0)
	associations := make([]legacyimport.AssociationObservation, 0)
	for _, observation := range observations {
		switch exact := observation.(type) {
		case legacyimport.CarrierObservation:
			carriers = append(carriers, exact)
		case legacyimport.AssociationObservation:
			associations = append(associations, exact)
		default:
			return nil, fmt.Errorf(
				"legacy SQLite observation %T is unsupported",
				observation,
			)
		}
	}
	if len(carriers) == len(observations) {
		return legacyimport.NewCarrierOnly(subject, carriers)
	}
	if len(associations) == len(observations) {
		return legacyimport.NewLegacyUnbound(subject, associations)
	}
	reason, err := legacyimport.NewUnresolvedReason(
		"mixed_legacy_observation_kinds",
	)
	if err != nil {
		return nil, err
	}
	return legacyimport.NewUnresolved(subject, reason, observations)
}

func encodedKey(
	row observedRow,
	columns []string,
) (string, error) {
	key := make([]canonicalKeyPart, 0, len(columns))
	for _, column := range columns {
		value, err := exactTextValue(row, column)
		if err != nil {
			return "", err
		}
		key = append(key, canonicalKeyPart{
			Column: column,
			Base64: encodeOpaque(value),
		})
	}
	encoded, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf(
			"encode legacy SQLite table %s key: %w",
			row.table,
			err,
		)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

type canonicalKeyPart struct {
	Column string `json:"column"`
	Base64 string `json:"base64"`
}

func exactTextValue(row observedRow, column string) ([]byte, error) {
	value, found := row.values[column]
	if !found {
		return nil, fmt.Errorf(
			"legacy SQLite table %s row lacks column %s",
			row.table,
			column,
		)
	}
	if value.storageClass != "text" {
		return nil, fmt.Errorf(
			"%w: table %s column %s uses storage class %s, want text",
			ErrUnsupportedLegacySchema,
			row.table,
			column,
			value.storageClass,
		)
	}
	return append([]byte(nil), value.bytes...), nil
}

func legacyObjectIdentity(raw []byte) (legacyimport.LegacyIdentityRef, error) {
	return legacyimport.NewLegacyIdentityRef(
		"legacy-object:" + encodeOpaque(raw),
	)
}

func encodeOpaque(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func quoteIdentifier(raw string) string {
	escaped := strings.ReplaceAll(raw, `"`, `""`)
	return `"` + escaped + `"`
}
