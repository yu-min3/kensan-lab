// Package ingredients conservatively interprets ingredient lines. Raw text is never discarded.
package ingredients

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"math/big"
	"regexp"
	"strings"
)

type Line struct {
	Raw         string `json:"raw"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // exact, range, vague, unparsed, group
	Min         string `json:"min,omitempty"`
	Max         string `json:"max,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Preparation string `json:"preparation,omitempty"`
}

const number = `([0-9]+(?:/[0-9]+|\.[0-9]+)?)`

var suffix = regexp.MustCompile(`^(.*?)\s*` + number + `(?:[〜～~-]` + number + `)?\s*(kg|g|ml|mL|L|個|本|枚|尾|切れ|片|束|袋|缶|玉|粒|株|パック|合|cups?|人分)?$`)
var spoon = regexp.MustCompile(`^(.*?)\s*(大さじ|小さじ|カップ)\s*` + number + `(?:[〜～~-]` + number + `)?$`)
var vague = regexp.MustCompile(`^(.*?)\s*(適量|少々|お好み(?:で)?)$`)
var prep = regexp.MustCompile(`^([^（(]+)[（(]([^）)]+)[）)]$`)
var group = regexp.MustCompile(`^(?:[A-Za-zＡ-Ｚ]|【[^】]+】|\[[^\]]+\]|(?:■|●|▼|◆).*)$`)
var digits = regexp.MustCompile(`[0-9０-９]`)
var validName = regexp.MustCompile(`[\p{L}]`)

func Canonical(name string) string {
	name = strings.TrimSpace(norm.NFKC.String(name))
	switch name {
	case "醤油":
		return "しょうゆ"
	case "たまねぎ", "玉葱":
		return "玉ねぎ"
	}
	return name
}
func Parse(raw string) Line {
	text := strings.TrimSpace(norm.NFKC.String(raw))
	line := Line{Raw: raw, Name: text, Kind: "unparsed"}
	if group.MatchString(text) && !digits.MatchString(text) && !vague.MatchString(text) {
		line.Kind = "group"
		return line
	}
	if strings.Contains(text, "または") || strings.Contains(text, " or ") || strings.Contains(text, "又は") {
		return line
	}
	if m := vague.FindStringSubmatch(text); m != nil && strings.TrimSpace(m[1]) != "" {
		line.Name = Canonical(m[1])
		line.Kind = "vague"
		line.Unit = m[2]
		return line
	}
	var name, min, max, unit string
	if m := spoon.FindStringSubmatch(text); m != nil {
		name, unit, min, max = m[1], m[2], m[3], m[4]
	} else if m := suffix.FindStringSubmatch(text); m != nil {
		name, min, max, unit = m[1], m[2], m[3], m[4]
	} else {
		return line
	}
	name = strings.TrimSpace(name)
	if !validName.MatchString(name) {
		return line
	}
	a, ok := new(big.Rat).SetString(min)
	if !ok || a.Sign() < 0 {
		return line
	}
	if max != "" {
		b, ok := new(big.Rat).SetString(max)
		if !ok || b.Cmp(a) < 0 {
			return line
		}
	}
	if m := prep.FindStringSubmatch(name); m != nil && regexp.MustCompile(`切|刻|すり|潰|薄|皮`).MatchString(m[2]) {
		name = m[1]
		line.Preparation = m[2]
	}
	line.Name = Canonical(name)
	line.Kind = "exact"
	line.Min = a.RatString()
	line.Unit = unit
	if max != "" {
		b, _ := new(big.Rat).SetString(max)
		line.Max = b.RatString()
		line.Kind = "range"
	}
	return line
}
func Multiply(value, scale string) (string, error) {
	a, ok := new(big.Rat).SetString(value)
	if !ok {
		return "", fmt.Errorf("数量を確認してください")
	}
	b, ok := new(big.Rat).SetString(scale)
	if !ok || b.Sign() <= 0 || b.Cmp(big.NewRat(100, 1)) > 0 {
		return "", fmt.Errorf("倍率は0より大きく100以下にしてください")
	}
	return new(big.Rat).Mul(a, b).RatString(), nil
}
func Add(a, b string) string {
	x, _ := new(big.Rat).SetString(a)
	y, _ := new(big.Rat).SetString(b)
	if x == nil || y == nil {
		return ""
	}
	return new(big.Rat).Add(x, y).RatString()
}
func Display(q string) string {
	r, ok := new(big.Rat).SetString(q)
	if !ok {
		return q
	}
	if r.IsInt() {
		return r.Num().String()
	}
	s := strings.TrimRight(strings.TrimRight(r.FloatString(3), "0"), ".")
	if s == "0" && r.Sign() != 0 {
		return q
	}
	return s
}
func (l Line) Scaled(scale string) (Line, error) {
	if _, e := Multiply("1", scale); e != nil {
		return l, e
	}
	var e error
	if l.Kind == "exact" || l.Kind == "range" {
		l.Min, e = Multiply(l.Min, scale)
		if e != nil {
			return l, e
		}
		if l.Max != "" {
			l.Max, e = Multiply(l.Max, scale)
		}
	}
	return l, e
}
func (l Line) Text() string {
	if l.Kind == "exact" {
		if l.Unit == "大さじ" || l.Unit == "小さじ" || l.Unit == "カップ" {
			return l.Name + " " + l.Unit + Display(l.Min)
		}
		return l.Name + " " + Display(l.Min) + l.Unit
	}
	if l.Kind == "range" {
		return l.Name + " " + Display(l.Min) + "〜" + Display(l.Max) + l.Unit
	}
	if l.Kind == "vague" {
		return l.Name + " " + l.Unit
	}
	return l.Raw
}
func (l Line) Key() string {
	key := l.Name + "\x00" + l.Unit + "\x00" + l.Kind
	if l.Kind == "unparsed" {
		key = l.Raw
	}
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:12])
}
