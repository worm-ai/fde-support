package knowledge

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/registry"
	"fde-support/internal/shared"
)

type Store struct {
	units []Unit
}

// FilterBySources returns a new Store containing only units from the specified source IDs.
// If sourceIDs is nil, the original store is returned unchanged (no filtering).
// If sourceIDs is empty (non-nil), an empty store is returned (explicit no sources).
func (s *Store) FilterBySources(sourceIDs []string) *Store {
	if sourceIDs == nil {
		return s
	}
	if len(sourceIDs) == 0 {
		return &Store{units: nil}
	}
	set := make(map[string]bool, len(sourceIDs))
	for _, id := range sourceIDs {
		set[id] = true
	}
	var filtered []Unit
	for _, unit := range s.units {
		if set[unit.SourceID] {
			filtered = append(filtered, unit)
		}
	}
	return &Store{units: filtered}
}

type Unit struct {
	SourceID  string
	SourceRef string
	Fields    map[string]any
	Content   string
}

type QualityReport struct {
	GeneratedAt                 time.Time           `json:"generatedAt"`
	ManifestFingerprint         string              `json:"manifestFingerprint"`
	KnowledgeConfigFingerprint  string              `json:"knowledgeConfigFingerprint"`
	KnowledgeSourcesFingerprint string              `json:"knowledgeSourcesFingerprint"`
	Sources                     []SourceReport      `json:"sources"`
	Status                      string              `json:"status"`
	Items                       []QualityReportItem `json:"items"`
}

type SourceReport struct {
	ID          string `json:"id"`
	URI         string `json:"uri"`
	ResolvedURI string `json:"resolvedUri"`
	SHA256      string `json:"sha256,omitempty"`
	Records     int    `json:"records"`
}

type QualityReportItem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Source   string `json:"source,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

type LoadOptions struct {
	WriteReport bool
}

func Load(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment) (*Store, *QualityReport, error) {
	return LoadWithOptions(ctx, m, env, LoadOptions{WriteReport: true})
}

func LoadWithOptions(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment, opts LoadOptions) (*Store, *QualityReport, error) {
	report := &QualityReport{
		GeneratedAt:                time.Now().UTC(),
		ManifestFingerprint:        fingerprintManifest(m),
		KnowledgeConfigFingerprint: fingerprintKnowledgeConfig(m),
		Status:                     "passed",
	}
	store := &Store{}
	for _, source := range m.Knowledge.Sources {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		if source.Type != "jsonl" && source.Type != "csv" && source.Type != "table" && source.Type != "rules" {
			item := QualityReportItem{Code: "UNSUPPORTED_KNOWLEDGE_SOURCE_TYPE", Severity: "block", Source: source.ID, Message: "M1 supports only jsonl knowledge sources"}
			report.Items = append(report.Items, item)
			continue
		}
		resolved := resolveManifestPath(m.BaseDir, source.URI)
		srcReport := SourceReport{ID: source.ID, URI: source.URI, ResolvedURI: resolved}
		units, hash, items := loadKnowledgeSource(source, resolved)
		srcReport.SHA256 = hash
		srcReport.Records = len(units)
		report.Sources = append(report.Sources, srcReport)
		report.Items = append(report.Items, items...)
		store.units = append(store.units, units...)
		if len(units) == 0 && !hasBlock(items) {
			report.Items = append(report.Items, QualityReportItem{Code: "KNOWLEDGE_SOURCE_EMPTY", Severity: "warn", Source: source.ID, Message: "knowledge source is empty"})
		}
		// Run quality gates against loaded units
		if len(m.Knowledge.QualityGates) > 0 {
			gateItems := evaluateQualityGates(source.ID, source.Schema, units, m.Knowledge.QualityGates)
			report.Items = append(report.Items, gateItems...)
		}
	}
	for _, item := range report.Items {
		if item.Severity == "block" {
			report.Status = "blocked"
			break
		}
	}
	report.KnowledgeSourcesFingerprint = fingerprintSourceReports(report.Sources)
	if opts.WriteReport {
		if err := writeReport(env.ReportPath(), report); err != nil {
			return nil, report, err
		}
	}
	if report.Status == "blocked" {
		return nil, report, shared.NewError("KNOWLEDGE_QUALITY_BLOCKED", env.ReportPath(), "knowledge_quality_passed has block findings")
	}
	return store, report, nil
}

func loadKnowledgeSource(source manifest.KnowledgeSourceSpec, resolved string) ([]Unit, string, []QualityReportItem) {
	switch source.Type {
	case "csv", "table":
		return loadCSVSource(source, resolved)
	case "rules":
		return loadJSONLSource(source, resolved)
	default:
		return loadJSONLSource(source, resolved)
	}
}

func (s *Store) Retrieve(ctx context.Context, query string, topK int) (registry.KnowledgeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return registry.KnowledgeResult{}, err
	}
	if topK <= 0 {
		topK = 5
	}
	type scored struct {
		unit  Unit
		score int
	}
	terms := tokenize(query)
	matches := make([]scored, 0, len(s.units))
	for _, unit := range s.units {
		score := 0
		contentLower := strings.ToLower(unit.Content)
		for _, term := range terms {
			if strings.Contains(contentLower, term) {
				score++
			}
		}
		if score == 0 && len(terms) == 0 {
			score = 1
		}
		if score > 0 {
			matches = append(matches, scored{unit: unit, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	if len(matches) > topK {
		matches = matches[:topK]
	}
	result := registry.KnowledgeResult{}
	for _, match := range matches {
		result.Passages = append(result.Passages, match.unit.Content)
		result.Citations = append(result.Citations, parseCitation(match.unit.SourceRef))
		result.Raw = append(result.Raw, match.unit.Fields)
	}
	return result, nil
}

func loadCSVSource(source manifest.KnowledgeSourceSpec, resolved string) ([]Unit, string, []QualityReportItem) {
	file, err := os.Open(resolved)
	if err != nil {
		return nil, "", []QualityReportItem{{Code: "KNOWLEDGE_SOURCE_MISSING", Severity: "block", Source: source.ID, Message: err.Error()}}
	}
	defer file.Close()

	hash := sha256.New()
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, "", []QualityReportItem{{Code: "KNOWLEDGE_CSV_READ_FAILED", Severity: "block", Source: source.ID, Message: err.Error()}}
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
		hash.Write([]byte(headers[i]))
	}

	var units []Unit
	var items []QualityReportItem
	lineNo := 1
	for {
		lineNo++
		row, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			items = append(items, QualityReportItem{Code: "KNOWLEDGE_CSV_READ_FAILED", Severity: "block", Source: source.ID, Line: lineNo, Message: err.Error()})
			break
		}
		record := map[string]any{}
		for i, header := range headers {
			if i < len(row) {
				value := strings.TrimSpace(row[i])
				record[header] = value
				hash.Write([]byte(value))
			}
		}
		sourceRef, _ := record["source_ref"].(string)
		if strings.TrimSpace(sourceRef) == "" {
			sourceRef = fmt.Sprintf("%s#row-%d", source.ID, lineNo-1)
			record["source_ref"] = sourceRef
		}
		content := recordContent(record)
		if strings.TrimSpace(content) == "" {
			items = append(items, QualityReportItem{Code: "KNOWLEDGE_TEXT_MISSING", Severity: "block", Source: source.ID, Line: lineNo, Message: "record has no searchable text field"})
			continue
		}
		units = append(units, Unit{SourceID: source.ID, SourceRef: sourceRef, Fields: record, Content: content})
	}
	return units, hex.EncodeToString(hash.Sum(nil)), items
}

// loadJSONLSourceValidation runs ValidateSchemaFields on JSONL units.

func loadJSONLSource(source manifest.KnowledgeSourceSpec, resolved string) ([]Unit, string, []QualityReportItem) {
	file, err := os.Open(resolved)
	if err != nil {
		return nil, "", []QualityReportItem{{Code: "KNOWLEDGE_SOURCE_MISSING", Severity: "block", Source: source.ID, Message: err.Error()}}
	}
	defer file.Close()
	hash := sha256.New()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var units []Unit
	var items []QualityReportItem
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		hash.Write([]byte(line))
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			items = append(items, QualityReportItem{Code: "KNOWLEDGE_JSONL_INVALID", Severity: "block", Source: source.ID, Line: lineNo, Message: err.Error()})
			continue
		}
		sourceRef, ok := record["source_ref"].(string)
		if !ok || strings.TrimSpace(sourceRef) == "" {
			items = append(items, QualityReportItem{Code: "KNOWLEDGE_SOURCE_REF_MISSING", Severity: "block", Source: source.ID, Line: lineNo, Message: "record is missing source_ref"})
			continue
		}
		content := recordContent(record)
		if strings.TrimSpace(content) == "" {
			items = append(items, QualityReportItem{Code: "KNOWLEDGE_TEXT_MISSING", Severity: "block", Source: source.ID, Line: lineNo, Message: "record has no searchable text field"})
			continue
		}
		units = append(units, Unit{SourceID: source.ID, SourceRef: sourceRef, Fields: record, Content: content})
	}
	if err := scanner.Err(); err != nil {
		items = append(items, QualityReportItem{Code: "KNOWLEDGE_JSONL_READ_FAILED", Severity: "block", Source: source.ID, Message: err.Error()})
	}
	return units, hex.EncodeToString(hash.Sum(nil)), items
}

func recordContent(record map[string]any) string {
	preferred := []string{"answer", "resolution", "question", "symptom", "cause", "description", "content"}
	var parts []string
	for _, key := range preferred {
		if value, ok := record[key].(string); ok && strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	for key, value := range record {
		if key == "source_ref" {
			continue
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	return strings.Join(parts, " ")
}

func tokenize(query string) []string {
	lower := strings.ToLower(query)
	replacer := strings.NewReplacer("?", " ", "？", " ", ".", " ", "。", " ", ",", " ", "，", " ", "!", " ", "！", " ", ":", " ", "：", " ")
	lower = replacer.Replace(lower)
	fields := strings.Fields(lower)
	seen := map[string]bool{}
	var out []string
	for _, field := range fields {
		if len([]rune(field)) <= 1 {
			continue
		}
		if !seen[field] {
			seen[field] = true
			out = append(out, field)
		}
	}
	return out
}

func parseCitation(sourceRef string) registry.Citation {
	if source, ref, ok := strings.Cut(sourceRef, "#"); ok {
		return registry.Citation{Source: source, Ref: ref}
	}
	return registry.Citation{Source: sourceRef, Ref: ""}
}

func writeReport(path string, report *QualityReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, bytes)
}

// atomicWriteFile writes data to a temporary file and atomically renames it into place.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fingerprintManifest(m *manifest.SolutionManifest) string {
	clone := *m
	clone.Path = ""
	clone.BaseDir = ""
	bytes, _ := json.Marshal(clone)
	// Normalize numeric values: json.Marshal uses consistent formatting for YAML-decoded numbers.
	// For production use, consider canonical CBOR or a stable serialization format.
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func FingerprintManifest(m *manifest.SolutionManifest) string {
	return fingerprintManifest(m)
}

func FingerprintKnowledgeConfig(m *manifest.SolutionManifest) string {
	return fingerprintKnowledgeConfig(m)
}

func FingerprintSourceReports(sources []SourceReport) string {
	return fingerprintSourceReports(sources)
}

func fingerprintKnowledgeConfig(m *manifest.SolutionManifest) string {
	payload := struct {
		Sources  []manifest.KnowledgeSourceSpec
		Schemas  []manifest.KnowledgeSchemaSpec
		Gates    []manifest.QualityGateSpec
		Bindings []manifest.KnowledgeBindingSpec
	}{
		Sources:  m.Knowledge.Sources,
		Schemas:  m.Knowledge.Schemas,
		Gates:    m.Knowledge.QualityGates,
		Bindings: m.Runtime.KnowledgeBindings,
	}
	bytes, _ := json.Marshal(payload)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func fingerprintSourceReports(sources []SourceReport) string {
	bytes, _ := json.Marshal(sources)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func resolveManifestPath(baseDir, uri string) string {
	if filepath.IsAbs(uri) {
		return filepath.Clean(uri)
	}
	return filepath.Clean(filepath.Join(baseDir, uri))
}

func hasBlock(items []QualityReportItem) bool {
	for _, item := range items {
		if item.Severity == "block" {
			return true
		}
	}
	return false
}

func (s *Store) Count() int {
	return len(s.units)
}

func (s *Store) String() string {
	return fmt.Sprintf("knowledge.Store{%d units}", len(s.units))
}


// validateSchemaFields checks that units satisfy the required fields declared by schema(s).
// It returns quality items for any missing required fields.
func validateSchemaFields(units []Unit, schemas []manifest.KnowledgeSchemaSpec, sourceID string, sourceSchemaID string) []QualityReportItem {
	if sourceSchemaID == "" || len(schemas) == 0 {
		return nil
	}
	var items []QualityReportItem
	var schema *manifest.KnowledgeSchemaSpec
	for i := range schemas {
		if schemas[i].ID == sourceSchemaID {
			schema = &schemas[i]
			break
		}
	}
	if schema == nil {
		return nil
	}
	for i, unit := range units {
		for _, field := range schema.Fields {
			if _, ok := unit.Fields[field]; !ok {
				items = append(items, QualityReportItem{
					Code:     "MISSING_REQUIRED_FIELD",
					Severity: "block",
					Source:   sourceID,
					Line:     i + 1,
					Message:  fmt.Sprintf("record is missing required field %q", field),
				})
			}
		}
	}
	return items
}

// evaluateQualityGates runs quality gate checks against loaded units and returns items.
func evaluateQualityGates(sourceID string, sourceSchema string, units []Unit, gates []manifest.QualityGateSpec) []QualityReportItem {
	// Scope filtering is handled per-gate below
	var items []QualityReportItem

	for _, gate := range gates {
		// Scope filtering: if gate has scope, only check units matching the scope schema
		if len(gate.Scope) > 0 {
			matched := false
			for _, scopeSchema := range gate.Scope {
				if scopeSchema == sourceSchema {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if gate.Type != "stale_content" || gate.MaxAgeDays <= 0 {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -gate.MaxAgeDays)
		for _, unit := range units {
			ts := extractTimestamp(unit.Fields)
			if ts.IsZero() {
				items = append(items, QualityReportItem{
					Code: "STALE_CONTENT", Severity: gate.Severity, Source: sourceID,
					Message: fmt.Sprintf("record %q has no parseable timestamp, treated as stale", unit.SourceRef),
				})
				continue
			}
			if ts.Before(cutoff) {
				items = append(items, QualityReportItem{
					Code: "STALE_CONTENT", Severity: gate.Severity, Source: sourceID,
					Message: fmt.Sprintf("record %q is stale (last updated %s, max age %d days)", unit.SourceRef, ts.Format(time.RFC3339), gate.MaxAgeDays),
				})
			}
		}
	}

	// conflicting_answers: check for duplicate answers to the same question
	seen := map[string]int{}
	for i, unit := range units {
		question := ""
		if q, ok := unit.Fields["question"].(string); ok {
			question = strings.TrimSpace(q)
		}
		if question == "" {
			continue
		}
		if prevIdx, exists := seen[question]; exists {
			prevAnswer := ""
			if a, ok := units[prevIdx].Fields["answer"].(string); ok {
				prevAnswer = strings.TrimSpace(a)
			}
			currAnswer := ""
			if a, ok := unit.Fields["answer"].(string); ok {
				currAnswer = strings.TrimSpace(a)
			}
			if prevAnswer != currAnswer && prevAnswer != "" && currAnswer != "" {
				items = append(items, QualityReportItem{
					Code: "CONFLICTING_ANSWERS", Severity: "warn", Source: sourceID,
					Message: fmt.Sprintf("conflicting answer for question %q", question),
				})
			}
		} else {
			seen[question] = i
		}
	}

	return items
}
