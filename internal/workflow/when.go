package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"fde-support/internal/shared"
)

type FieldRef struct {
	NodeID string
	Field  string
}

type Literal struct {
	Value any
	Kind  string
}

type WhenCondition struct {
	Raw   string
	Left  FieldRef
	Op    string
	Right Literal
}

var whenPattern = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_-]*)\.([A-Za-z_][A-Za-z0-9_-]*)\s*(==|!=|<=|>=|<|>)\s*(.+?)\s*$`)

func ParseWhen(expr string) (*WhenCondition, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, fmt.Errorf("when expression is empty")
	}
	if strings.Contains(expr, "&&") || strings.Contains(expr, "||") || strings.Contains(expr, "[") || strings.Contains(expr, "]") || strings.Contains(expr, "(") || strings.Contains(expr, ")") {
		return nil, fmt.Errorf("when expression only supports one comparison")
	}
	matches := whenPattern.FindStringSubmatch(expr)
	if matches == nil {
		return nil, fmt.Errorf("when expression must be node_id.field <op> literal")
	}
	nodeID := matches[1]
	if nodeID == "inputs" || nodeID == "request" || nodeID == "signal" || nodeID == "runtime_request" {
		return nil, fmt.Errorf("when expression must reference upstream node output, got %q", nodeID)
	}
	lit, err := parseLiteral(matches[4])
	if err != nil {
		return nil, err
	}
	if (matches[3] == "<" || matches[3] == "<=" || matches[3] == ">" || matches[3] == ">=") && lit.Kind != "number" {
		return nil, fmt.Errorf("operator %q requires numeric literal", matches[3])
	}
	return &WhenCondition{
		Raw:   expr,
		Left:  FieldRef{NodeID: nodeID, Field: matches[2]},
		Op:    matches[3],
		Right: lit,
	}, nil
}

func parseLiteral(raw string) (Literal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "true" {
		return Literal{Value: true, Kind: "boolean"}, nil
	}
	if raw == "false" {
		return Literal{Value: false, Kind: "boolean"}, nil
	}
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			unquoted, err := strconv.Unquote(raw)
			if err != nil {
				if raw[0] == '\'' {
					return Literal{Value: raw[1 : len(raw)-1], Kind: "string"}, nil
				}
				return Literal{}, fmt.Errorf("invalid string literal: %w", err)
			}
			return Literal{Value: unquoted, Kind: "string"}, nil
		}
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return Literal{Value: n, Kind: "number"}, nil
	}
	return Literal{}, fmt.Errorf("unsupported literal %q", raw)
}

func (c *WhenCondition) Evaluate(outputs map[string]map[string]any) (bool, error) {
	nodeOutput, ok := outputs[c.Left.NodeID]
	if !ok {
		return false, fmt.Errorf("missing upstream node output %q", c.Left.NodeID)
	}
	left, ok := nodeOutput[c.Left.Field]
	if !ok {
		return false, fmt.Errorf("missing upstream field %s.%s", c.Left.NodeID, c.Left.Field)
	}
	switch c.Right.Kind {
	case "string":
		v, ok := left.(string)
		if !ok {
			return false, fmt.Errorf("field %s.%s is not a string", c.Left.NodeID, c.Left.Field)
		}
		return compareString(v, c.Op, c.Right.Value.(string))
	case "boolean":
		v, ok := left.(bool)
		if !ok {
			return false, fmt.Errorf("field %s.%s is not a boolean", c.Left.NodeID, c.Left.Field)
		}
		return compareBool(v, c.Op, c.Right.Value.(bool))
	case "number":
		v, ok := shared.ToFloat64(left)
		if !ok {
			return false, fmt.Errorf("field %s.%s is not a number", c.Left.NodeID, c.Left.Field)
		}
		return compareNumber(v, c.Op, c.Right.Value.(float64))
	default:
		return false, fmt.Errorf("unsupported literal kind %q", c.Right.Kind)
	}
}

func compareString(left, op, right string) (bool, error) {
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	default:
		return false, fmt.Errorf("operator %q is not supported for strings", op)
	}
}

func compareBool(left bool, op string, right bool) (bool, error) {
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	default:
		return false, fmt.Errorf("operator %q is not supported for booleans", op)
	}
}

func compareNumber(left float64, op string, right float64) (bool, error) {
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "<":
		return left < right, nil
	case "<=":
		return left <= right, nil
	case ">":
		return left > right, nil
	case ">=":
		return left >= right, nil
	default:
		return false, fmt.Errorf("operator %q is not supported", op)
	}
}
