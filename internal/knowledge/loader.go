package knowledge

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type Unit struct {
	SourceID  string
	SourceRef string
	Fields    map[string]any
	Content   string
}

type QualityReport struct {
	GeneratedAt         time.Time           `json:"generatedAt"`
	ManifestFingerprint string              `json:"manifestFingerprint"`
	Sources             []SourceReport      `json:"sources"`
	Status              string              `json:"status"`
	Items               []QualityReportItem `json:"items"`
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

func Load(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment) (*Store, *QualityReport, error) {
	report := &QualityReport{
		GeneratedAt:         time.Now().UTC(),
		ManifestFingerprint: fingerprintManifest(m),
		Status:              "passed",
	}
	store := &Store{}
	for _, source := range m.Knowledge.Sources {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		if source.Type != "jsonl" {
			item := QualityReportItem{Code: "UNSUPPORTED_KNOWLEDGE_SOURCE_TYPE", Severity: "block", Source: source.ID, Message: "M1 supports only jsonl knowledge sources"}
			report.Items = append(report.Items, item)
			continue
		}
		resolved := resolveManifestPath(m.BaseDir, source.URI)
		srcReport := SourceReport{ID: source.ID, URI: source.URI, ResolvedURI: resolved}
		units, hash, items := loadJSONLSource(source, resolved)
		srcReport.SHA256 = hash
		srcReport.Records = len(units)
		report.Sources = append(report.Sources, srcReport)
		report.Items = append(report.Items, items...)
		store.units = append(store.units, units...)
		if len(units) == 0 && !hasBlock(items) {
			report.Items = append(report.Items, QualityReportItem{Code: "KNOWLEDGE_SOURCE_EMPTY", Severity: "warn", Source: source.ID, Message: "knowledge source is empty"})
		}
	}
	for _, item := range report.Items {
		if item.Severity == "block" {
			report.Status = "blocked"
			break
		}
	}
	if err := writeReport(env.ReportPath(), report); err != nil {
		return nil, report, err
	}
	if report.Status == "blocked" {
		return nil, report, shared.NewError("KNOWLEDGE_QUALITY_BLOCKED", env.ReportPath(), "knowledge_quality_passed has block findings")
	}
	return store, report, nil
}

func (s *Store) Retrieve(ctx context.Context, query string, topK int) (registry.KnowledgeResult, error) {
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
	tmp := path + ".tmp"
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fingerprintManifest(m *manifest.SolutionManifest) string {
	clone := *m
	clone.Path = ""
	clone.BaseDir = ""
	bytes, _ := json.Marshal(clone)
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
