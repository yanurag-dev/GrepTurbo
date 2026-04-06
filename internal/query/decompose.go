package query

import (
	"regexp/syntax"

	"fastregex/internal/trigram"
)

// Result holds the trigrams extracted from a regex pattern.
// If Wildcard is true, no trigrams could be extracted and the query
// must fall back to scanning every file.
type Result struct {
	Trigrams []trigram.T
	Wildcard bool
}

// Decompose parses pattern and extracts the trigrams that must appear
// in any file matching the regex.
//
// Rules:
//   - Literals produce their overlapping trigrams (intersected into the set)
//   - Alternations (foo|bar) produce the union of each branch's trigrams
//   - Wildcards (.*, .+, ., [a-z], etc.) force Wildcard=true for that branch
//   - If any required branch is a wildcard, the whole result is Wildcard=true
func Decompose(pattern string) (*Result, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	re = re.Simplify()
	return extract(re), nil
}

// extract recursively walks the regex AST and returns the trigram Result
// for the subtree rooted at re.
func extract(re *syntax.Regexp) *Result {
	switch re.Op {

	case syntax.OpLiteral:
		// A literal string like "func" or "Error".
		// Convert runes to a string and extract trigrams.
		s := string(re.Rune)
		ts := trigram.Extract(s)
		if len(ts) == 0 {
			// Literal shorter than 3 chars — not useful for filtering
			return &Result{Wildcard: true}
		}
		return &Result{Trigrams: ts}

	case syntax.OpConcat:
		// A sequence: each sub-expression must all match in order.
		// Collect trigrams from ALL sub-expressions (intersect: file must
		// satisfy every part). If any part is a wildcard, we still keep
		// trigrams from the non-wildcard parts.
		result := &Result{}
		for _, sub := range re.Sub {
			r := extract(sub)
			if !r.Wildcard {
				result.Trigrams = append(result.Trigrams, r.Trigrams...)
			}
		}
		// If we got no trigrams at all from any sub-expression, it's a wildcard
		if len(result.Trigrams) == 0 {
			result.Wildcard = true
		}
		return result

	case syntax.OpAlternate:
		// foo|bar — file must match at least one branch.
		// We can only filter on trigrams that appear in EVERY branch.
		// If any branch is a wildcard, the whole alternation is a wildcard
		// (a file that doesn't contain "foo"'s trigrams might still match via "bar").
		var all [][]trigram.T
		for _, sub := range re.Sub {
			r := extract(sub)
			if r.Wildcard {
				return &Result{Wildcard: true}
			}
			all = append(all, r.Trigrams)
		}
		// Union: include trigrams from all branches.
		// (A file is a candidate if it might match any branch.)
		seen := make(map[trigram.T]struct{})
		var union []trigram.T
		for _, ts := range all {
			for _, t := range ts {
				if _, ok := seen[t]; !ok {
					seen[t] = struct{}{}
					union = append(union, t)
				}
			}
		}
		if len(union) == 0 {
			return &Result{Wildcard: true}
		}
		return &Result{Trigrams: union}

	case syntax.OpCapture:
		// Capturing group — transparent, just recurse into the single child
		if len(re.Sub) == 1 {
			return extract(re.Sub[0])
		}
		return &Result{Wildcard: true}

	case syntax.OpRepeat:
		// {n,m} repetition — recurse into the repeated sub-expression
		if len(re.Sub) == 1 && re.Min >= 1 {
			return extract(re.Sub[0])
		}
		return &Result{Wildcard: true}

	default:
		// OpStar, OpPlus, OpQuest, OpAnyChar, OpAnyCharNotNL,
		// OpCharClass, OpBeginText, OpEndText, etc.
		// None of these let us require specific trigrams.
		return &Result{Wildcard: true}
	}
}
