package report

import (
	"strconv"
	"strings"
	"unicode"
)

// cents parses a decimal money string ("−50.00", "1.5") into integer cents.
// Inputs come from Postgres numeric ::text casts, so they are well-formed;
// malformed input returns 0.
func cents(s string) int64 {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0
	}
	c := w*100 + f
	if neg {
		c = -c
	}
	return c
}

// centsToDec renders integer cents as a plain decimal string ("-50.00").
func centsToDec(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	s := strconv.FormatInt(c/100, 10) + "." + pad2(c%100)
	if neg {
		return "-" + s
	}
	return s
}

// usd renders integer cents as "$1,234.56" (negatives as "-$…").
func usd(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole := strconv.FormatInt(c/100, 10)
	var b strings.Builder
	lead := len(whole) % 3
	if lead == 0 {
		lead = 3
	}
	b.WriteString(whole[:lead])
	for i := lead; i < len(whole); i += 3 {
		b.WriteByte(',')
		b.WriteString(whole[i : i+3])
	}
	out := "$" + b.String() + "." + pad2(c%100)
	if neg {
		return "-" + out
	}
	return out
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// titleCase lowercases then capitalizes the first rune of each word.
func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
