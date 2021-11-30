package main

import (
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
		if err := stashTemplateTags(path); err != nil {
			log.Fatalf("%s: %s", path, err)
		}
	}
}

// stashTemplateTags opens a given YAML file and stashes template tags that
// would not be recognized by kustomize. The tags are remembered, so they can
// be resored after calling kustomize.
func stashTemplateTags(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, doc := range splitYamlByDocument(b) {
		root := &yaml.Node{}
		if err := yaml.Unmarshal(doc, root); err != nil {
			return err
		}
		err = stashNode([]*yaml.Node{}, root)
		if err != nil {
			return err
		}
		log.Println("---")
	}
	return nil
}

// stashNode walks through yaml.Node object and its Content recursively to
// find, mark, and temporarily remove template tags.
func stashNode(parents []*yaml.Node, y *yaml.Node) error {
	nodeDetails(parents, y)
	subParents := append(parents, y)
	for _, subNode := range y.Content {
		stashNode(subParents, subNode)
	}
	return nil
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
