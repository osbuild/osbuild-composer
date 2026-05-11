// Package yamlplus extends the standard YAML parser with custom tags for cross-referencing
// anchors and documents across multiple YAML files.
//
// The primary feature is the !xref tag, which allows you to reference anchors defined in
// other YAML files that have been registered with a Loader. This is particularly useful
// for managing configuration files that share common settings or for breaking large
// YAML configurations into smaller, reusable pieces.
//
// # Basic Usage
//
// Create a Loader with a filesystem, register YAML files, and unmarshal data that
// contains !xref tags:
//
//	loader := yamlplus.NewLoader(os.DirFS("config"))
//	loader.RegisterFile("base.yaml")
//	loader.RegisterFile("database.yaml")
//
//	var config map[string]any
//	data := []byte(`
//	  db: !xref "database.yaml"
//	  network: !xref "base.yaml#network"
//	`)
//	loader.Unmarshal(data, &config)
//
// # The !xref Tag
//
// The !xref tag supports two forms:
//
//  1. Reference an entire file: !xref "filename.yaml"
//     Returns the first document in the file.
//
//  2. Reference a specific anchor: !xref "filename.yaml#anchorname"
//     Returns the node with that anchor.
//
// # Map Merges
//
// The !xref tag works with YAML map merge syntax (<<):
//
//	config:
//	  <<: !xref "base.yaml#defaults"
//	  port: 8080  # overrides the merged value
//
// You can also merge multiple sources:
//
//	config:
//	  <<:
//	    - !xref "base.yaml#defaults"
//	    - !xref "network.yaml#settings"
//	  timeout: 30s
//
// # Path-Based Namespacing
//
// Files are registered by their exact path. References must use the same path:
//
//	loader.RegisterFile("configs/app.yaml")
//	// Must reference as "configs/app.yaml", not "app.yaml"
//	data: !xref "configs/app.yaml"
//
// # Circular Dependency Detection
//
// The loader detects and reports circular dependencies:
//
//	// a.yaml: !xref "b.yaml"
//	// b.yaml: !xref "a.yaml"
//	// Error: circular dependency detected
//
// # Thread Safety
//
// A Loader is safe for concurrent Unmarshal calls after all files have been registered.
// However, RegisterFile, RegisterDirectory, and RegisterRecursively are not safe to
// call concurrently with each other or with Unmarshal.
package yamlplus

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Loader processes YAML files and resolves !xref tags by maintaining a registry
// of anchors from all registered files. A Loader is associated with a single
// filesystem and can register multiple YAML files for cross-referencing.
//
// The Filesystem field is exported but should not be modified after creation.
// The anchorRegistry is internal and tracks all discovered anchors using the
// format "filepath#anchorname" for named anchors, or just "filepath" for
// document-level references.
type Loader struct {
	// Filesystem is the filesystem from which YAML files are loaded.
	// It should not be modified after the Loader is created.
	Filesystem fs.FS

	anchorRegistry map[string]*yaml.Node
}

// NewLoader creates a new Loader that reads YAML files from the given filesystem.
// The filesystem is typically created with os.DirFS for directory-based access
// or can be an in-memory filesystem for testing.
//
// Example:
//
//	loader := yamlplus.NewLoader(os.DirFS("/etc/config"))
//	loader.RegisterFile("app.yaml")
func NewLoader(f fs.FS) *Loader {
	return &Loader{
		Filesystem:     f,
		anchorRegistry: make(map[string]*yaml.Node),
	}
}

// Decoder reads and decodes YAML values from an input stream, resolving
// !xref tags using the Loader's anchor registry.
//
// A Decoder is created with [Loader.NewDecoder] and mirrors the API of
// [go.yaml.in/yaml/v3.Decoder], adding cross-file reference resolution.
type Decoder struct {
	loader      *Loader
	decoder     *yaml.Decoder
	knownFields bool
}

// NewDecoder creates a new Decoder that reads from r and resolves !xref
// tags using the loader's registered anchors.
//
// The returned Decoder supports all the same options as [go.yaml.in/yaml/v3.Decoder],
// including [Decoder.KnownFields] for strict field checking.
//
// Example:
//
//	dec := loader.NewDecoder(reader)
//	dec.KnownFields(true)
//	var config Config
//	err := dec.Decode(&config)
func (l *Loader) NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		loader:  l,
		decoder: yaml.NewDecoder(r),
	}
}

// KnownFields configures whether the Decoder should fail on unknown fields.
// When enabled, the Decoder will return an error if the YAML document contains
// keys that do not map to exported fields in the target struct.
//
// This mirrors [go.yaml.in/yaml/v3.Decoder.KnownFields].
func (d *Decoder) KnownFields(enable bool) {
	d.knownFields = enable
}

// Decode reads the next YAML document from the Decoder's stream, resolves
// all !xref tags, and stores the result in the value pointed to by out.
//
// Successive calls to Decode read successive documents from the stream.
// When the stream is exhausted, Decode returns [io.EOF].
//
// See [Loader.Unmarshal] for details on !xref resolution behavior.
func (d *Decoder) Decode(out any) error {
	var root yaml.Node
	if err := d.decoder.Decode(&root); err != nil {
		return err
	}

	stack := make(map[string]bool)
	if err := d.loader.replaceXrefs(&root, stack); err != nil {
		return err
	}

	if d.knownFields {
		data, err := yaml.Marshal(&root)
		if err != nil {
			return err
		}

		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		return dec.Decode(out)
	}

	return root.Decode(out)
}

// RegisterFile reads a YAML file and registers all its anchors for cross-referencing.
// The file is identified by its path relative to the Loader's filesystem.
//
// The entire first document is also registered using just the path as the key,
// allowing references like !xref "config.yaml" to return the whole document.
//
// If the file contains multiple documents, all anchors from all documents are registered,
// but document-level references (without #anchor) return only the first document.
//
// RegisterFile can be called multiple times with the same path, which will overwrite
// previous registrations. This is not recommended.
//
// Returns an error if the file cannot be opened, read, or parsed as valid YAML.
func (l *Loader) RegisterFile(path string) error {
	f, err := l.Filesystem.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var root yaml.Node
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&root); err != nil {
		return err
	}

	if len(root.Content) > 0 {
		// set up an anchor directly to the document so !xref "somefile.yaml"
		// works
		l.anchorRegistry[path] = root.Content[0]

		for _, doc := range root.Content {
			l.scanAnchors(path, doc)
		}
	}

	return nil
}

// RegisterDirectory registers all YAML files in the specified directory.
// Only files with .yaml or .yml extensions (case-insensitive) are registered.
// Subdirectories are not traversed; use RegisterRecursively for recursive registration.
//
// Example:
//
//	loader.RegisterDirectory("configs")  // registers configs/*.yaml and configs/*.yml
//
// Returns an error if the directory cannot be read or if any YAML file fails to register.
func (l *Loader) RegisterDirectory(dir string) error {
	entries, err := fs.ReadDir(l.Filesystem, dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && hasYAMLSuffix(entry.Name()) {
			fullPath := path.Join(dir, entry.Name())

			if err := l.RegisterFile(fullPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// RegisterRecursively walks the directory tree starting at dir and registers all
// YAML files found. Only files with .yaml or .yml extensions (case-insensitive)
// are registered.
//
// Example:
//
//	loader.RegisterRecursively("configs")  // registers all YAML files in configs and subdirectories
//
// Returns an error if the directory walk fails or if any YAML file fails to register.
func (l *Loader) RegisterRecursively(dir string) error {
	return fs.WalkDir(l.Filesystem, dir, func(currentPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && hasYAMLSuffix(entry.Name()) {
			if err := l.RegisterFile(currentPath); err != nil {
				return err
			}
		}

		return nil
	})
}

// Unmarshal parses YAML data and resolves all !xref tags using the loader's
// anchor registry, then unmarshals the result into out.
//
// The data may contain !xref tags that reference anchors in files previously
// registered with RegisterFile, RegisterDirectory, or RegisterRecursively.
//
// References can be:
//   - Document-level: !xref "file.yaml"
//   - Anchor-specific: !xref "file.yaml#anchorname"
//
// The !xref tag also works in map merge contexts:
//
//	config:
//	  <<: !xref "base.yaml#defaults"
//	  port: 8080
//
// Circular dependencies are detected and returned as errors.
//
// Returns an error if:
//   - The YAML syntax is invalid
//   - An !xref references a non-existent file or anchor
//   - A circular dependency is detected
//   - The data cannot be unmarshaled into out
func (l *Loader) Unmarshal(data []byte, out any) error {
	return l.NewDecoder(bytes.NewReader(data)).Decode(out)
}

// Go through a YAML node and its children and replace occurrences of nodes that
// are tagged with `!xref` with the node they refer to
func (l *Loader) replaceXrefs(node *yaml.Node, stack map[string]bool) error {
	if node == nil {
		return nil
	}

	if node.Tag == "!xref" {
		// we can directly return in this case since resolveDirectXRef
		// recurses
		return l.resolveDirectXRef(node, stack)
	}

	if node.Kind == yaml.MappingNode {
		if err := l.resolveMapMergeXRef(node, stack); err != nil {
			return err
		}
	}

	for _, child := range node.Content {
		if err := l.replaceXrefs(child, stack); err != nil {
			return err
		}
	}

	return nil
}

// Get a (previously registered) anchor by reference.
func (l *Loader) getAnchor(ref string) (*yaml.Node, error) {
	if target, ok := l.anchorRegistry[ref]; ok {
		return target, nil
	}

	return nil, fmt.Errorf("xref %q not found in registry", ref)
}

// Scan a node and its children for any anchors and store them in the
// anchor registry. The path is included for namespacing.
func (l *Loader) scanAnchors(path string, node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Anchor != "" {
		l.anchorRegistry[fmt.Sprintf("%s#%s", path, node.Anchor)] = node
	}

	for _, child := range node.Content {
		l.scanAnchors(path, child)
	}
}

func (l *Loader) applyMerge(src, dst *yaml.Node) int {
	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return 0
	}

	existingKeys := make(map[string]bool)
	for i := 0; i < len(dst.Content); i += 2 {
		existingKeys[dst.Content[i].Value] = true
	}

	newKeyVals := make([]*yaml.Node, 0)
	for i := 0; i < len(src.Content); i += 2 {
		key := src.Content[i]
		val := src.Content[i+1]
		if !existingKeys[key.Value] {
			newKeyVals = append(newKeyVals, key, val)
		}
	}

	dst.Content = append(newKeyVals, dst.Content...)

	return len(newKeyVals)
}

func (l *Loader) resolveDirectXRef(node *yaml.Node, stack map[string]bool) error {
	// Save the original reference before we overwrite node.Value
	originalRef := node.Value

	if stack[originalRef] {
		return fmt.Errorf("circular dependency detected: %q", originalRef)
	}

	stack[originalRef] = true

	resolved, err := l.getAnchor(originalRef)
	if err != nil {
		return err
	}

	// for a direct reference to a document we want its first child (mapping
	// or sequence) instead of the DocumentNode itself. note that when files
	// are registered we verify that they are non-empty so one can never
	// reference an empty document
	if resolved.Kind == yaml.DocumentNode {
		resolved = resolved.Content[0]
	}

	clone := cloneNode(resolved)

	node.Kind = clone.Kind
	node.Style = clone.Style
	node.Tag = clone.Tag
	node.Value = clone.Value
	node.Alias = clone.Alias
	node.Content = clone.Content

	// Keep the reference in stack during recursive processing to detect cycles,
	// then remove it after processing is complete
	err = l.replaceXrefs(node, stack)
	delete(stack, originalRef)

	return err
}

func (l *Loader) resolveMapMergeXRef(node *yaml.Node, stack map[string]bool) error {
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]

		if key.Value == "<<" {
			toMerge, err := l.extractMapMergeTargets(val)
			if err != nil {
				return err
			}

			if len(toMerge) > 0 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				newKeys := 0

				// apply in sequence order, the yaml spec says that earlier items in the sequence
				// take precedence
				for _, src := range toMerge {
					var resolved *yaml.Node

					if src.Tag == "!xref" {
						if stack[src.Value] {
							return fmt.Errorf("circular dependency detected: %q", src.Value)
						}

						stack[src.Value] = true

						var err error

						resolved, err = l.getAnchor(src.Value)
						if err != nil {
							return err
						}

						if resolved.Kind == yaml.DocumentNode {
							resolved = resolved.Content[0]
						}

						// Clone before modifying to avoid mutating the anchor registry
						resolved = cloneNode(resolved)

						if err := l.replaceXrefs(resolved, stack); err != nil {
							return err
						}

						delete(stack, src.Value)
					} else {
						resolved = src
						if err := l.replaceXrefs(resolved, stack); err != nil {
							return err
						}
					}

					newKeys += l.applyMerge(resolved, node)
				}

				i = i + newKeys - 2
			}
		}
	}

	return nil
}

func (l *Loader) extractMapMergeTargets(val *yaml.Node) ([]*yaml.Node, error) {
	var targets []*yaml.Node

	// Follow direct alias before processing
	actualVal := val
	if val.Kind == yaml.AliasNode && val.Alias != nil {
		actualVal = val.Alias
	}

	if actualVal.Tag == "!xref" || actualVal.Kind == yaml.MappingNode {
		targets = append(targets, actualVal)
		return targets, nil
	}

	if actualVal.Kind == yaml.SequenceNode {
		for _, item := range actualVal.Content {
			actualItem := item
			if item.Kind == yaml.AliasNode && item.Alias != nil {
				actualItem = item.Alias
			}
			if actualItem.Tag == "!xref" || actualItem.Kind == yaml.MappingNode {
				targets = append(targets, actualItem)
			}
		}
	}
	return targets, nil
}

func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}

	oldToNew := make(map[*yaml.Node]*yaml.Node)
	clone := cloneNodeWithMap(n, oldToNew)

	// fix up alias pointers to point to cloned nodes
	fixAliases(clone, oldToNew)

	return clone
}

func cloneNodeWithMap(n *yaml.Node, oldToNew map[*yaml.Node]*yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}

	clone := *n       // copy value types (Kind, Style, Tag, Value, etc.)
	clone.Anchor = "" // Clear anchor name - cloned nodes shouldn't carry over anchor identities
	clonePtr := &clone
	oldToNew[n] = clonePtr

	if n.Content != nil {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			clone.Content[i] = cloneNodeWithMap(child, oldToNew)
		}
	}

	return clonePtr
}

func fixAliases(n *yaml.Node, oldToNew map[*yaml.Node]*yaml.Node) {
	if n == nil {
		return
	}

	// If this node has an Alias pointer, remap it to the cloned version
	if n.Alias != nil {
		if newAlias, ok := oldToNew[n.Alias]; ok {
			n.Alias = newAlias
		}
	}

	// Recursively fix aliases in children
	for _, child := range n.Content {
		fixAliases(child, oldToNew)
	}
}

func hasYAMLSuffix(name string) bool {
	suffix := strings.ToLower(path.Ext(name))
	return suffix == ".yaml" || suffix == ".yml"
}
