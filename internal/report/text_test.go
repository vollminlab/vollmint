package report

import "testing"

func TestCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1", 100},
		{"1.5", 150},
		{"1.50", 150},
		{"-50.00", -5000},
		{"-0.01", -1},
		{"1234.56", 123456},
		{"128.42", 12842},
	}
	for _, c := range cases {
		if got := cents(c.in); got != c.want {
			t.Errorf("cents(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestCentsToDec(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{150, "1.50"},
		{-5000, "-50.00"},
		{-1, "-0.01"},
		{123456, "1234.56"},
	}
	for _, c := range cases {
		if got := centsToDec(c.in); got != c.want {
			t.Errorf("centsToDec(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUSD(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "$0.00"},
		{150, "$1.50"},
		{123456, "$1,234.56"},
		{100000000, "$1,000,000.00"},
		{-5000, "-$50.00"},
		{12345678, "$123,456.78"},
		{12300, "$123.00"},
	}
	for _, c := range cases {
		if got := usd(c.in); got != c.want {
			t.Errorf("usd(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTitleCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"VERIZON WIRELESS", "Verizon Wireless"},
		{"netflix", "Netflix"},
		{"", ""},
		{"APPLE TV+", "Apple Tv+"},
	}
	for _, c := range cases {
		if got := titleCase(c.in); got != c.want {
			t.Errorf("titleCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
