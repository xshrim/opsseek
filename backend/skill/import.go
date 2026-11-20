package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"opsseek/backend/authorization"
)

// Import payload limits (R001 §3.2 / §3.2.1 / §3.2.2).
const (
	ImportFormatOpsSeek     = "" // opsseek.skill/v1 document: json/yaml/markdown
	ImportFormatSkillMD     = "skillmd"
	ImportFormatZip         = "zip"
	importMaxPayloadBytes   = 10 << 20 // 10MB per import request
	importMaxUnpackedBytes  = 50 << 20 // 50MB total unpacked
	importMaxZipEntries     = 200
	importMaxSkillsPerZip   = 20
	importMaxSkillMDBytes   = 256 << 10 // SKILL.md including frontmatter
	importMaxFrontmatter    = 64 << 10
	importMaxBodyBytes      = 100 << 10
	importMaxReferenceLines = 500
	importMaxYAMLDepth      = 100
	importCompressionRatio  = 100
)

const (
	ImportStatusImported = "imported"
	ImportStatusFailed   = "failed"
)

type ImportInput struct {
	ScopeID  string
	Format   string
	Data     []byte
	Category string
	ActorID  string
}

type ImportItemResult struct {
	Status       string   `json:"status"`
	Identifier   string   `json:"identifier"`
	SkillID      string   `json:"skill_id,omitempty"`
	VersionID    string   `json:"version_id,omitempty"`
	SourceDigest string   `json:"source_digest,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Import converts external Skill payloads (OpsSeek documents, Agent Skills
// SKILL.md text or zip packages) into draft Skill versions. Package level
// violations (zip-slip, symlinks, bombs, limits) reject the whole request;
// per-item problems are reported in the per-item results.
func (s *Service) Import(ctx context.Context, input ImportInput) ([]ImportItemResult, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.Category = strings.TrimSpace(input.Category)
	if input.ScopeID == "" {
		return nil, invalid("scope_id is required")
	}
	if !allowsScope(ctx, input.ScopeID) {
		return nil, authorization.ErrForbidden
	}
	if len(input.Data) > importMaxPayloadBytes {
		return nil, invalid(fmt.Sprintf("import payload exceeds %d bytes", importMaxPayloadBytes))
	}
	switch input.Category {
	case "":
		input.Category = CategoryDiagnosis
	case CategoryDiagnosis, CategoryMonitoring, CategoryOptimization, CategoryMaintenance:
	default:
		return nil, invalid("Skill metadata.category is invalid")
	}

	switch strings.ToLower(strings.TrimSpace(input.Format)) {
	case BodyFormatJSON, BodyFormatYAML, BodyFormatMarkdown:
		item := s.importDocument(ctx, input.ActorID, input.ScopeID, strings.ToLower(strings.TrimSpace(input.Format)), input.Data)
		return []ImportItemResult{item}, nil
	case ImportFormatSkillMD:
		return []ImportItemResult{s.importAgentSkill(ctx, input.ActorID, input.ScopeID, input.Category, "pasted SKILL.md", input.Data, nil)}, nil
	case ImportFormatZip:
		return s.importZip(ctx, input)
	default:
		return nil, invalid("format must be json, yaml, markdown, skillmd or zip")
	}
}

func (s *Service) importDocument(ctx context.Context, actorID, scopeID, format string, raw []byte) ImportItemResult {
	parsed, err := ParseDocument(format, raw)
	if err != nil {
		return ImportItemResult{Status: ImportStatusFailed, Error: err.Error()}
	}
	meta := importMetadata{Document: parsed.Document}
	return s.persistImport(ctx, actorID, scopeID, meta, parsed.ContentSHA256, nil)
}

type importMetadata struct {
	Document Document
}

func (s *Service) importAgentSkill(ctx context.Context, actorID, scopeID, category, source string, raw []byte, references []referenceFile) ImportItemResult {
	document, warnings, err := convertAgentSkill(raw, actorID, category, source, references)
	if err != nil {
		return ImportItemResult{Status: ImportStatusFailed, Error: err.Error(), Warnings: warnings}
	}
	normalized, err := json.Marshal(document)
	if err != nil {
		return ImportItemResult{Status: ImportStatusFailed, Error: fmt.Sprintf("normalize imported Skill: %v", err)}
	}
	digest := sha256Hex(normalized)
	return s.persistImport(ctx, actorID, scopeID, importMetadata{Document: document}, digest, warnings)
}

func (s *Service) persistImport(ctx context.Context, actorID, scopeID string, meta importMetadata, digest string, warnings []string) ImportItemResult {
	doc := meta.Document
	item, err := documentToCreateVersionInput(doc, digest, actorID)
	if err != nil {
		return ImportItemResult{Status: ImportStatusFailed, Identifier: doc.Metadata.Identifier, Error: err.Error(), Warnings: warnings}
	}
	item.SkillID, err = s.importSkillID(ctx, scopeID, doc)
	if err != nil {
		return ImportItemResult{Status: ImportStatusFailed, Identifier: doc.Metadata.Identifier, Error: err.Error(), Warnings: warnings}
	}
	version, created, err := s.store.ImportVersion(ctx, *item)
	if err != nil {
		message := err.Error()
		if errors.Is(err, ErrConflict) {
			message = fmt.Sprintf("Skill %s already has a version string %q; imported versions must use a new exact version string", doc.Metadata.Identifier, doc.Metadata.Version)
		}
		return ImportItemResult{Status: ImportStatusFailed, Identifier: doc.Metadata.Identifier, Error: message, Warnings: warnings}
	}
	status := ImportStatusImported
	if !created {
		status = "skipped"
		if warnings == nil {
			warnings = []string{"identical content was imported before; returning the existing version"}
		}
	}
	return ImportItemResult{
		Status:       status,
		Identifier:   doc.Metadata.Identifier,
		SkillID:      version.SkillID,
		VersionID:    version.ID,
		SourceDigest: digest,
		Warnings:     warnings,
	}
}

func (s *Service) importSkillID(ctx context.Context, scopeID string, doc Document) (string, error) {
	existing, err := s.store.FindSkillByIdentifier(ctx, scopeID, doc.Metadata.Identifier)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	created, err := s.store.CreateSkill(ctx, Skill{
		ScopeID:    scopeID,
		Name:       doc.Metadata.Name,
		Identifier: doc.Metadata.Identifier,
		Category:   doc.Metadata.Category,
		Tags:       doc.Metadata.Tags,
		Maintainer: doc.Metadata.Maintainer,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func documentToCreateVersionInput(doc Document, digest, actorID string) (*CreateVersionInput, error) {
	description, _ := doc.Spec["description"].(string)
	body := bodyInstruction(doc.Spec["body"])
	tools := make([]ToolSpec, 0)
	if rawTools, ok := doc.Spec["tools"].([]any); ok {
		for _, entry := range rawTools {
			fields, ok := entry.(map[string]any)
			if !ok {
				return nil, invalid("Skill spec.tools items must be objects")
			}
			name, _ := fields["name"].(string)
			if !allowedToolName(strings.TrimSpace(name)) {
				return nil, invalid(fmt.Sprintf("Skill declares unsupported tool %q; imported tools must exactly match the platform whitelist", name))
			}
			spec := ToolSpec{Name: strings.TrimSpace(name)}
			if desc, ok := fields["description"].(string); ok {
				spec.Description = desc
			}
			if schema, ok := fields["input_schema"]; ok {
				encoded, err := json.Marshal(schema)
				if err != nil {
					return nil, invalid(fmt.Sprintf("Skill tool %q has an invalid input_schema", name))
				}
				spec.InputSchema = encoded
			}
			tools = append(tools, spec)
		}
	}
	if len(tools) > 20 {
		return nil, invalid("a Skill version may declare at most 20 tools")
	}
	inputSchema := json.RawMessage(`{"type":"object"}`)
	if schema, ok := doc.Spec["input_schema"]; ok && schema != nil {
		encoded, err := json.Marshal(schema)
		if err != nil {
			return nil, invalid("Skill spec.input_schema must be a JSON object schema")
		}
		inputSchema = encoded
	}
	outputSchema := json.RawMessage(`{"type":"object"}`)
	if schema, ok := doc.Spec["output_schema"]; ok && schema != nil {
		encoded, err := json.Marshal(schema)
		if err != nil {
			return nil, invalid("Skill spec.output_schema must be a JSON object schema")
		}
		outputSchema = encoded
	}
	return &CreateVersionInput{
		Manifest: Manifest{
			Name:        doc.Metadata.Name,
			Description: description,
			Instruction: body,
			Version:     doc.Metadata.Version,
		},
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Tools:        tools,
		RiskLevel:    "read_only",
		CreatedBy:    actorID,
		SourceType:   "imported",
		SourceDigest: digest,
	}, nil
}

func bodyInstruction(body any) string {
	switch item := body.(type) {
	case string:
		return item
	case map[string]any:
		encoded, err := json.Marshal(item)
		if err == nil {
			return string(encoded)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Agent Skills conversion
// ---------------------------------------------------------------------------

type referenceFile struct {
	Name     string
	Content  []byte
	Warnings []string
}

var (
	agentSkillNamePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	scriptTagPattern       = regexp.MustCompile(`(?is)<\s*script\b.*?<\s*/\s*script\s*>`)
	javascriptURLPattern   = regexp.MustCompile(`(?i)javascript\s*:`)
	injectionScanPatterns  = []string{"ignore previous", "ignore all previous", "disregard previous", "system prompt", "reveal your", "api key", "password", "credential", "secret"}
	agentSkillMetadataKeys = map[string]bool{"version": true, "author": true, "domain": true}
)

func convertAgentSkill(raw []byte, actorID, category, source string, references []referenceFile) (Document, []string, error) {
	warnings := make([]string, 0)
	if len(raw) > importMaxSkillMDBytes {
		return Document{}, warnings, invalid(fmt.Sprintf("SKILL.md exceeds %d bytes", importMaxSkillMDBytes))
	}
	text := strings.ReplaceAll(strings.TrimPrefix(string(raw), "\ufeff"), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Document{}, warnings, invalid("SKILL.md requires YAML front matter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return Document{}, warnings, invalid("SKILL.md front matter is not closed")
	}
	frontmatter := []byte(strings.Join(lines[1:closing], "\n"))
	if len(frontmatter) > importMaxFrontmatter {
		return Document{}, warnings, invalid(fmt.Sprintf("SKILL.md front matter exceeds %d bytes", importMaxFrontmatter))
	}
	fields, err := restrictedYAML(frontmatter)
	if err != nil {
		return Document{}, warnings, invalid(fmt.Sprintf("invalid SKILL.md front matter: %v", err))
	}

	name, _ := fields["name"].(string)
	name = strings.TrimSpace(name)
	if !agentSkillNamePattern.MatchString(name) {
		return Document{}, warnings, invalid("SKILL.md name must be 3-64 lowercase letters, digits or hyphens")
	}
	description, _ := fields["description"].(string)
	description = strings.TrimSpace(description)
	if description == "" || len([]rune(description)) > 1024 {
		return Document{}, warnings, invalid("SKILL.md description must be 1 to 1024 characters")
	}
	if len([]rune(description)) > 4000 {
		description = string([]rune(description)[:4000])
	}
	if strings.Contains(source, "/") {
		base := path.Base(strings.TrimSuffix(strings.ReplaceAll(source, `\`, "/"), "/"))
		if base != "SKILL.md" && base != name {
			warnings = append(warnings, fmt.Sprintf("directory name %q does not match skill name %q", base, name))
		}
	}

	version := "0.1.0"
	maintainer := "imported:" + actorID
	tags := make([]string, 0, 4)
	if metadata, ok := fields["metadata"].(map[string]any); ok {
		for key, value := range metadata {
			text, isString := value.(string)
			switch {
			case key == "version":
				version = strings.TrimSpace(text)
			case key == "author":
				maintainer = strings.TrimSpace(text)
			case !isString:
				warnings = append(warnings, fmt.Sprintf("metadata.%s has a non-string value and was dropped", key))
			case agentSkillMetadataKeys[key]:
				tags = append(tags, key+":"+text)
			default:
				warnings = append(warnings, fmt.Sprintf("unsupported metadata key %q was dropped", key))
			}
		}
	}
	if license, ok := fields["license"].(string); ok && strings.TrimSpace(license) != "" {
		warnings = append(warnings, "license "+strings.TrimSpace(license)+" is recorded in the import report only")
	}
	if _, ok := fields["compatibility"]; ok {
		warnings = append(warnings, "compatibility has no OpsSeek mapping and was dropped")
	}

	tools := make([]any, 0)
	if declared, exists := fields["allowed-tools"]; exists {
		text, isString := declared.(string)
		if !isString {
			return Document{}, warnings, invalid("SKILL.md allowed-tools must be a space separated string")
		}
		for _, token := range strings.Fields(text) {
			if !allowedToolName(token) {
				return Document{}, warnings, invalid(fmt.Sprintf("SKILL.md allowed-tools declares %q which is not in the platform tool whitelist", token))
			}
			tools = append(tools, map[string]any{"name": token})
		}
	}

	body := strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	if len(references) > 0 {
		builder := strings.Builder{}
		builder.WriteString(body)
		for _, reference := range references {
			builder.WriteString("\n\n## Reference: " + reference.Name + "\n\n")
			builder.Write(reference.Content)
			warnings = append(warnings, reference.Warnings...)
		}
		body = builder.String()
	}
	if len(body) > importMaxBodyBytes {
		return Document{}, warnings, invalid(fmt.Sprintf("Skill body exceeds %d bytes", importMaxBodyBytes))
	}
	sanitized, changed := sanitizeBody(body)
	if changed {
		warnings = append(warnings, "body contained script tags or javascript: URLs which were removed")
	}
	for _, pattern := range injectionScanPatterns {
		if strings.Contains(strings.ToLower(sanitized), pattern) {
			warnings = append(warnings, fmt.Sprintf("body mentions %q; review before publishing", pattern))
		}
	}
	if strings.TrimSpace(sanitized) == "" {
		return Document{}, warnings, invalid("SKILL.md body is required")
	}
	if category == "" {
		category = CategoryDiagnosis
	}

	spec := map[string]any{
		"description":   description,
		"input_schema":  map[string]any{"type": "object"},
		"output_schema": map[string]any{"type": "object"},
		"body":          sanitized,
	}
	if len(tools) > 0 {
		spec["tools"] = tools
	}
	document := Document{
		APIVersion: DocumentAPIVersion,
		Kind:       DocumentKind,
		Metadata: Metadata{
			Name:       name,
			Identifier: name,
			Version:    version,
			Category:   category,
			Tags:       tags,
			Maintainer: maintainer,
		},
		Spec: spec,
	}
	if err := validateDocument(document); err != nil {
		return Document{}, warnings, err
	}
	return document, warnings, nil
}

func sanitizeBody(body string) (string, bool) {
	cleaned := scriptTagPattern.ReplaceAllString(body, "")
	cleaned = javascriptURLPattern.ReplaceAllString(cleaned, "blocked:")
	return cleaned, cleaned != body
}

// restrictedYAML parses YAML with import specific hardening: the input is
// size limited by the caller, aliases are rejected outright, nesting depth is
// capped and the result must be a string keyed object (R001 §3.2.2).
func restrictedYAML(raw []byte) (map[string]any, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, err
	}
	if len(node.Content) == 0 {
		return nil, fmt.Errorf("front matter is empty")
	}
	if err := walkRestricted(node.Content[0], 1); err != nil {
		return nil, err
	}
	var value any
	if err := node.Content[0].Decode(&value); err != nil {
		return nil, err
	}
	object, err := yamlObject(value)
	if err != nil {
		return nil, err
	}
	result, ok := object.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("front matter must be an object")
	}
	for key := range result {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("front matter keys must be non-empty")
		}
	}
	return result, nil
}

func walkRestricted(node *yaml.Node, depth int) error {
	if depth > importMaxYAMLDepth {
		return fmt.Errorf("front matter nesting exceeds %d levels", importMaxYAMLDepth)
	}
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("front matter aliases are not allowed")
	}
	for _, child := range node.Content {
		if err := walkRestricted(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agent Skills zip packages
// ---------------------------------------------------------------------------

type zipSkillPackage struct {
	Dir        string
	SkillMD    []byte
	References []referenceFile
}

func (s *Service) importZip(ctx context.Context, input ImportInput) ([]ImportItemResult, error) {
	packages, droppedWarnings, err := parseSkillZip(input.Data)
	if err != nil {
		return nil, err
	}
	results := make([]ImportItemResult, 0, len(packages))
	for _, pkg := range packages {
		warnings := append([]string{}, droppedWarnings...)
		item := s.importAgentSkill(ctx, input.ActorID, input.ScopeID, input.Category, pkg.Dir, pkg.SkillMD, pkg.References)
		item.Warnings = append(warnings, item.Warnings...)
		results = append(results, item)
	}
	if len(results) == 0 {
		return nil, invalid("zip package contains no SKILL.md")
	}
	return results, nil
}

// parseSkillZip unpacks an Agent Skills zip entirely in memory: entries are
// never written to disk, so extraction cannot escape the process. Zip-slip
// paths, symlinks, nested archives, compression bombs and entry limits are
// rejected for the whole package (R001 §3.2.1).
func parseSkillZip(data []byte) ([]zipSkillPackage, []string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, invalid("invalid zip package")
	}
	if len(reader.File) > importMaxZipEntries {
		return nil, nil, invalid(fmt.Sprintf("zip package exceeds %d entries", importMaxZipEntries))
	}
	warnings := make([]string, 0)
	files := make(map[string][]byte, len(reader.File))
	unpacked := 0
	for _, file := range reader.File {
		name := file.Name
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		if strings.Contains(name, `\`) || filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return nil, nil, invalid(fmt.Sprintf("zip entry %q uses an absolute or backslash path", name))
		}
		cleaned := path.Clean(name)
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			return nil, nil, invalid(fmt.Sprintf("zip entry %q escapes the package root", name))
		}
		if file.Mode()&os.ModeSymlink != 0 || file.Mode()&os.ModeDevice != 0 {
			return nil, nil, invalid(fmt.Sprintf("zip entry %q is a symlink or device", name))
		}
		if strings.EqualFold(filepath.Ext(name), ".zip") || strings.EqualFold(filepath.Ext(name), ".tar") || strings.EqualFold(filepath.Ext(name), ".gz") {
			warnings = append(warnings, "nested archive "+name+" was not extracted")
			continue
		}
		lower := strings.ToLower(cleaned)
		switch {
		case strings.Contains(lower, "/scripts/"), strings.HasPrefix(lower, "scripts/"),
			strings.Contains(lower, "/assets/"), strings.HasPrefix(lower, "assets/"):
			warnings = append(warnings, "executable or asset entry "+name+" was dropped and is never executed")
			continue
		case !strings.HasSuffix(lower, ".md"):
			continue
		}
		content, err := readZipEntry(file, &unpacked)
		if err != nil {
			return nil, nil, err
		}
		files[cleaned] = content
	}
	packages := make([]zipSkillPackage, 0)
	seen := make(map[string]bool, len(files))
	for name, content := range files {
		if path.Base(name) != "SKILL.md" || seen[dirKey(name)] {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			dir = ""
		}
		if len(packages) >= importMaxSkillsPerZip {
			return nil, nil, invalid(fmt.Sprintf("zip package exceeds %d skills", importMaxSkillsPerZip))
		}
		seen[dirKey(name)] = true
		packages = append(packages, zipSkillPackage{Dir: dir, SkillMD: content, References: collectReferences(files, dir, &warnings)})
	}
	sortPackages(packages)
	return packages, warnings, nil
}

func collectReferences(files map[string][]byte, dir string, warnings *[]string) []referenceFile {
	prefix := ""
	if dir != "" {
		prefix = dir + "/references/"
	} else {
		prefix = "references/"
	}
	references := make([]referenceFile, 0)
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(strings.ToLower(name), ".md") {
			names = append(names, name)
		}
	}
	sortStrings(names)
	totalLines := 0
	for _, name := range names {
		content := files[name]
		lineCount := strings.Count(string(content), "\n") + 1
		if lineCount > importMaxReferenceLines || totalLines+lineCount > importMaxReferenceLines {
			*warnings = append(*warnings, "reference "+name+" exceeds the inline line budget and was dropped")
			continue
		}
		totalLines += lineCount
		references = append(references, referenceFile{Name: path.Base(name), Content: content})
	}
	return references
}

func readZipEntry(file *zip.File, unpacked *int) ([]byte, error) {
	if file.UncompressedSize64 > uint64(importMaxUnpackedBytes) {
		return nil, invalid(fmt.Sprintf("zip entry %q exceeds the unpacked size limit", file.Name))
	}
	if file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > importCompressionRatio {
		return nil, invalid(fmt.Sprintf("zip entry %q exceeds the %dx compression ratio limit", file.Name, importCompressionRatio))
	}
	limit := int64(importMaxSkillMDBytes)
	reader, err := file.Open()
	if err != nil {
		return nil, invalid(fmt.Sprintf("zip entry %q cannot be read", file.Name))
	}
	defer func() { _ = reader.Close() }()
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, invalid(fmt.Sprintf("zip entry %q cannot be read", file.Name))
	}
	if int64(len(content)) > limit {
		return nil, invalid(fmt.Sprintf("zip entry %q exceeds %d bytes", file.Name, limit))
	}
	// Defense in depth for entries whose declared size lies about the stream:
	// drain up to one extra byte to detect truncation-based bombs.
	probe := make([]byte, 1)
	if n, _ := io.ReadFull(reader, probe); n > 0 {
		content = append(content, probe[:n]...)
		if int64(len(content)) > limit {
			return nil, invalid(fmt.Sprintf("zip entry %q exceeds %d bytes", file.Name, limit))
		}
	}
	*unpacked += len(content)
	if *unpacked > importMaxUnpackedBytes {
		return nil, invalid("zip package exceeds the total unpacked size limit")
	}
	return content, nil
}

func dirKey(name string) string {
	dir := path.Dir(name)
	if dir == "." {
		return ""
	}
	return dir
}

func sortPackages(packages []zipSkillPackage) {
	sort.Slice(packages, func(i, j int) bool { return packages[i].Dir < packages[j].Dir })
}

func sortStrings(items []string) {
	sort.Strings(items)
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
