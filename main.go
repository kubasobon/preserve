package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

const yamlDocSeparator = "\n---\n"

var files = []string{
	"app/resources.yaml",
}

func main() {
	for _, path := range files {
		if err := stashTemplateTagsInFile(path); err != nil {
			log.Fatalf("%s: %s", path, err)
		}
	}
}

// stashTemplateTagsInFile opens a given YAML file and stashes template tags that
// would not be recognized by kustomize. The tags are remembered, so they can
// be resored after calling kustomize.
func stashTemplateTagsInFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, doc := range splitYamlByDocument(b) {
		root := &yaml.Node{}
		if err := yaml.Unmarshal(doc, root); err != nil {
			return err
		}

		if !isK8sObject(root) {
			continue
		}

		docId, err := getDocIdentifier(root)
		if err != nil {
			return err
		}
		log.Println(path, ": ", docId)

		err = stashTemplateTagsInDoc([]*yaml.Node{}, root)
		if err != nil {
			return err
		}
		log.Println("---")
	}
	return nil
}

// stashTemplateTagsInDoc walks through yaml.Node object and its Content
// recursively to find, mark, and temporarily remove template tags.
func stashTemplateTagsInDoc(parents []*yaml.Node, node *yaml.Node) error {
	// nodeDetails(parents, y)
	subParents := append(parents, node)

	for _, subNode := range node.Content {
		stashTemplateTagsInDoc(subParents, subNode)
	}
	return nil
}

// getNodePath produces a path that represents node's position in a document in
// a way that is unique to it. Order of nodes in the source YAML file can be
// reshuffled (save for arrays, or yaml.SequenceNodes as they're called in the
// upstream library) and the path will remain valid. getNodePath is based on
// assumption that parents[0] node is a valid K8s object -
// `isK8sObject(parents[0])` should return `true`.
func getNodePath(parents []*yaml.Node, node *yaml.Node) (string, error) {
	path := ""

	return path, nil
}

// TODO: check for the below specified in kustomization:
// - namespace
// - namePrefix
// - nameSuffix
// and apply here!
// doc: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#kustomize-feature-list
func getDocIdentifier(root *yaml.Node) (string, error) {
	var apiVersion, kind, name, namespace string
	doc := root.Content[0]

	{
		_, node, ok := findInMappingNode(doc, "apiVersion")
		if !ok {
			return "", fmt.Errorf("could not read apiVersion")
		}
		apiVersion = node.Value
	}
	{
		_, node, ok := findInMappingNode(doc, "kind")
		if !ok {
			return "", fmt.Errorf("could not read kind")
		}
		kind = node.Value
	}
	var metadata *yaml.Node
	{
		_, meta, ok := findInMappingNode(doc, "metadata")
		if !ok {
			return "", fmt.Errorf("could not read metadata")
		}
		metadata = meta
	}
	{
		_, node, ok := findInMappingNode(metadata, "name")
		if !ok {
			return "", fmt.Errorf("could not read metadata.name")
		}
		name = node.Value
	}
	{
		_, node, ok := findInMappingNode(metadata, "namespace")
		if ok {
			namespace = node.Value
		}
	}

	id := fmt.Sprintf("%s.%s", apiVersion, kind)
	if namespace != "" {
		id = fmt.Sprintf("%s.%s/%s", id, namespace, name)
	} else {
		id = fmt.Sprintf("%s.%s", id, name)
	}
	return id, nil
}

func findInMappingNode(mappingNode *yaml.Node, key string) (idx int, node *yaml.Node, found bool) {
	if mappingNode.Kind != yaml.MappingNode {
		return 0, nil, false
	}
	for i, n := range mappingNode.Content {
		if n.Value == key {
			// Found the key node. Will return the next node, since
			// MappingNode.Content is structured like this:
			// [key1, value1, key2, value2, ...]
			return i + 1, mappingNode.Content[i+1], true
		}
	}
	return 0, nil, false
}

func findKeyInMappingNode(mappingNode *yaml.Node, key *yaml.Node) (idx int, mapKey string, found bool) {
	if mappingNode.Kind != yaml.MappingNode {
		return 0, "", false
	}
	for i, n := range mappingNode.Content {
		if n.Kind == key.Kind && n.Value == key.Value && eqContent(n, key) {
			return i - 1, mappingNode.Content[i-1].Value, true
		}
	}
	return 0, "", false
}

func findInSequenceNode(sequenceNode *yaml.Node, key *yaml.Node) (idx int, found bool) {
	if sequenceNode.Kind != yaml.SequenceNode {
		return 0, false
	}
	for i, n := range sequenceNode.Content {
		if n.Kind == key.Kind && n.Value == key.Value && eqContent(n, key) {
			return i, true
		}
	}
	return 0, false
}

func eqContent(a, b *yaml.Node) bool {
	if (a == nil && b != nil) || (a != nil && b == nil) {
		return false
	} else if a == nil && b == nil {
		return true
	}

	if (a.Content == nil && b.Content != nil) || (a.Content != nil && b.Content == nil) {
		return false
	} else if a.Content == nil && b.Content == nil {
		return true
	}

	if len(a.Content) != len(b.Content) {
		return false
	}

	for i, _ := range a.Content {
		if a.Content[i] != b.Content[i] {
			return false
		}
	}

	return true
}

func nodeDetails(parents []*yaml.Node, y *yaml.Node) {
	comments := ""
	if y.HeadComment != "" {
		comments += " #h"
	}
	if y.LineComment != "" {
		comments += " #l"
	}
	if y.FootComment != "" {
		comments += " #f"
	}
	kind := map[yaml.Kind]string{
		yaml.DocumentNode: "DocumentNode",
		yaml.SequenceNode: "SequenceNode",
		yaml.MappingNode:  "MappingNode",
		yaml.ScalarNode:   "ScalarNode",
		yaml.AliasNode:    "AliasNode",
	}[y.Kind]
	indent := strings.Repeat("  ", len(parents))
	log.Printf(
		"%s[%s] %s: %s (@%d:%d) %s\n", indent, kind, y.Tag, y.Value, y.Line, y.Column, comments,
	)
}

func splitYamlByDocument(b []byte) [][]byte {
	documents := strings.Split(string(b), yamlDocSeparator)
	byteDocuments := [][]byte{}
	for _, doc := range documents {
		if len(doc) == 0 {
			continue
		}
		byteDocuments = append(byteDocuments, []byte(doc))
	}
	return byteDocuments
}

// isK8sObject checks if given node represents a K8s resource. Since mapping is
// addressed by group, kind, namespace, and name, we will skip yaml documents
// not representing valid objects.
func isK8sObject(node *yaml.Node) bool {
	if node.Kind != yaml.DocumentNode {
		return false
	}

	if len(node.Content) != 1 {
		return false
	}

	if node.Content[0].Kind != yaml.MappingNode {
		return false
	}

	var apiVersionPresent, kindPresent, metadataPresent bool
	for _, subNode := range node.Content[0].Content {
		switch subNode.Value {
		case "apiVersion":
			apiVersionPresent = true
		case "kind":
			kindPresent = true
		case "metadata":
			metadataPresent = true
		}
		if apiVersionPresent && kindPresent && metadataPresent {
			break
		}
	}

	if !apiVersionPresent || !kindPresent || !metadataPresent {
		return false
	}

	return true
}
