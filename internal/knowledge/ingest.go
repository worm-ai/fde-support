package knowledge

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/shared"
)

// IngestResult holds the outcome of a knowledge ingestion run.
type IngestResult struct {
	SourceID  string              `json:"sourceId"`
	Type      string              `json:"type"`
	URI       string              `json:"uri"`
	Records   int                 `json:"records"`
	Ingested  int                 `json:"ingested"`
	Blocked   int                 `json:"blocked"`
	Warnings  int                 `json:"warnings"`
	Status    string              `json:"status"`
	Items     []QualityReportItem `json:"items,omitempty"`
	StartedAt time.Time           `json:"startedAt"`
	EndedAt   time.Time           `json:"endedAt"`
}

// IngestReport is the overall ingestion report.
type IngestReport struct {
	GeneratedAt   time.Time      `json:"generatedAt"`
	Results       []IngestResult `json:"results"`
	Status        string         `json:"status"`
	TotalRecords  int            `json:"totalRecords"`
	TotalIngested int            `json:"totalIngested"`
}

// Ingest executes the knowledge ingestion pipeline: reads source files,
// validates records against quality gates, and produces a quality report.
func Ingest(ctx context.Context, m *manifest.SolutionManifest, env environment.ResolvedEnvironment) (*IngestReport, error) {
	report := &IngestReport{
		GeneratedAt: time.Now().UTC(),
		Status:      "passed",
	}

	for _, source := range m.Knowledge.Sources {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		result := ingestSource(ctx, m, source, m.Knowledge.QualityGates)
		report.Results = append(report.Results, result)
		report.TotalRecords += result.Records
		report.TotalIngested += result.Ingested
		if result.Status == "blocked" {
			report.Status = "blocked"
		}
	}

	if err := writeReport(env.ReportPath(), qualityReportFromIngest(m, report)); err != nil {
		return report, err
	}

	if report.Status == "blocked" {
		return report, shared.NewError("KNOWLEDGE_INGEST_BLOCKED", env.ReportPath(), "knowledge ingestion has block findings")
	}
	return report, nil
}

func qualityReportFromIngest(m *manifest.SolutionManifest, report *IngestReport) *QualityReport {
	quality := &QualityReport{
		GeneratedAt:                report.GeneratedAt,
		ManifestFingerprint:        FingerprintManifest(m),
		KnowledgeConfigFingerprint: FingerprintKnowledgeConfig(m),
		Status:                     report.Status,
	}
	for _, result := range report.Results {
		quality.Sources = append(quality.Sources, SourceReport{
			ID:          result.SourceID,
			URI:         result.URI,
			ResolvedURI: resolveManifestPath(m.BaseDir, result.URI),
			Records:     result.Records,
		})
		quality.Items = append(quality.Items, result.Items...)
	}
	quality.KnowledgeSourcesFingerprint = FingerprintSourceReports(quality.Sources)
	return quality
}

func ingestSource(ctx context.Context, m *manifest.SolutionManifest, source manifest.KnowledgeSourceSpec, gates []manifest.QualityGateSpec) IngestResult {
	result := IngestResult{
		SourceID:  source.ID,
		Type:      source.Type,
		URI:       source.URI,
		StartedAt: time.Now().UTC(),
		Status:    "passed",
	}
	defer func() { result.EndedAt = time.Now().UTC() }()

	resolved := resolveManifestPath(m.BaseDir, source.URI)

	switch source.Type {
	case "jsonl":
		return ingestJSONL(ctx, source, resolved, result, gates)
	case "csv", "table":
		return ingestCSV(ctx, source, resolved, result)
	case "markdown", "md":
		result.Status = "warning"
		result.Items = append(result.Items, QualityReportItem{
			Code:     "UNSUPPORTED_KNOWLEDGE_SOURCE_TYPE",
			Severity: "warn",
			Source:   source.ID,
			Message:  "markdown knowledge sources require Python Worker for processing",
		})
		return result
	default:
		result.Status = "blocked"
		result.Items = append(result.Items, QualityReportItem{
			Code:     "UNSUPPORTED_KNOWLEDGE_SOURCE_TYPE",
			Severity: "block",
			Source:   source.ID,
			Message:  fmt.Sprintf("unsupported knowledge source type: %s", source.Type),
		})
		return result
	}
}

func ingestJSONL(ctx context.Context, source manifest.KnowledgeSourceSpec, resolved string, result IngestResult, gates []manifest.QualityGateSpec) IngestResult {
	units, _, items := loadJSONLSource(source, resolved)
	result.Records = len(units)
	result.Items = items
	result.Ingested = len(units)

	for _, item := range items {
		if item.Severity == "block" {
			result.Blocked++
			result.Ingested--
		}
		if item.Severity == "warn" {
			result.Warnings++
		}
	}

	// Enhanced quality gate checks
	runQualityGates(source, units, &result, gates)

	if result.Blocked > 0 {
		result.Status = "blocked"
	} else if result.Warnings > 0 {
		result.Status = "warning"
	}
	return result
}

func ingestCSV(ctx context.Context, source manifest.KnowledgeSourceSpec, resolved string, result IngestResult) IngestResult {
	file, err := os.Open(resolved)
	if err != nil {
		result.Status = "blocked"
		result.Items = append(result.Items, QualityReportItem{
			Code: "KNOWLEDGE_SOURCE_MISSING", Severity: "block", Source: source.ID, Message: err.Error(),
		})
		return result
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		result.Status = "blocked"
		result.Items = append(result.Items, QualityReportItem{
			Code: "KNOWLEDGE_CSV_READ_FAILED", Severity: "block", Source: source.ID, Message: err.Error(),
		})
		return result
	}

	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	lineNo := 1
	for {
		lineNo++
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) == 0 || allEmpty(record) {
			continue
		}
		result.Records++
		result.Ingested++
	}

	if result.Records == 0 {
		result.Items = append(result.Items, QualityReportItem{
			Code: "KNOWLEDGE_SOURCE_EMPTY", Severity: "warn", Source: source.ID, Message: "CSV knowledge source is empty",
		})
	}
	return result
}

func runQualityGates(source manifest.KnowledgeSourceSpec, units []Unit, result *IngestResult, gates []manifest.QualityGateSpec) {
	items := evaluateQualityGates(source.ID, source.Schema, units, gates)
	result.Items = append(result.Items, items...)
	for _, item := range items {
		if item.Severity == "block" {
			result.Blocked++
		} else {
			result.Warnings++
		}
	}
}

func allEmpty(values []string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// extractTimestamp tries to parse a timestamp from common fields and formats.
func extractTimestamp(fields map[string]any) time.Time {
	timestampKeys := []string{"updatedAt", "updated_at", "createdAt", "created_at", "modifiedAt", "modified_at", "timestamp", "date"}
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
	for _, key := range timestampKeys {
		val, ok := fields[key]
		if !ok {
			continue
		}
		s, ok := val.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		s = strings.TrimSpace(s)
		for _, layout := range formats {
			if ts, err := time.Parse(layout, s); err == nil {
				return ts
			}
		}
	}
	return time.Time{}
}
