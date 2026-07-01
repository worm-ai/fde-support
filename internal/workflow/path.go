package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var pathSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func ParseSimplePath(path string) (string, string, error) {
	segments := strings.Split(path, ".")
	if len(segments) != 2 {
		return "", "", fmt.Errorf("path must contain exactly one root and one field")
	}
	for _, segment := range segments {
		if !pathSegmentPattern.MatchString(segment) {
			return "", "", fmt.Errorf("invalid path segment %q in path %q", segment, path)
		}
	}
	return segments[0], segments[1], nil
}

func ValidateSimplePath(path string, allowedRoots map[string]bool) error {
	segments := strings.Split(path, ".")
	if len(segments) < 2 {
		return fmt.Errorf("path must contain a root and field")
	}
	if !allowedRoots[segments[0]] {
		return fmt.Errorf("path root %q is not allowed", segments[0])
	}
	for _, segment := range segments {
		if !pathSegmentPattern.MatchString(segment) {
			return fmt.Errorf("invalid path segment %q", segment)
		}
	}
	return nil
}

func ResolvePath(root map[string]any, path string) (any, bool) {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return nil, false
	}
	var cur any = root
	for _, segment := range segments {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func ToMap(value any) (map[string]any, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func NodeInputRoots(inputs map[string]any, outputs map[string]map[string]any) map[string]any {
	root := map[string]any{"inputs": inputs}
	for nodeID, output := range outputs {
		root[nodeID] = output
	}
	return root
}
