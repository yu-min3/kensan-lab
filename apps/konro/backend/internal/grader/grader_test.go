package grader

import (
	"encoding/json"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalcases"
)

func load(t *testing.T, id string) evalcases.Case {
	t.Helper()
	s, err := evalcases.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range s.Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no case %s", id)
	return evalcases.Case{}
}

func out(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestFaithfulOutputPasses(t *testing.T) {
	c := load(t, "basic-nikujaga")
	r := Grade(c, "succeeded", "", out(map[string]any{"title": "肉じゃが", "servings": "2人分", "ingredients": []string{"じゃがいも 3個", "牛こま切れ肉 150g", "玉ねぎ 1個", "醤油 大さじ2", "砂糖 大さじ1", "水 200ml"}, "steps": []string{"じゃがいもと玉ねぎを一口大に切る。", "鍋で肉を炒め、野菜を加えて炒める。", "水・砂糖・醤油を入れて落とし蓋をし、15分煮る。"}, "assessment": "読めた"}))
	if r.Status != Passed || r.Score != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestProductionGateRejectsNewNumbers(t *testing.T) {
	c := load(t, "basic-nikujaga")
	r := Grade(c, "succeeded", "", out(map[string]any{"title": "肉じゃが", "servings": "2人分", "ingredients": []string{"じゃがいも 4個"}, "steps": []string{"30分煮る"}, "assessment": ""}))
	if r.Status != Failed || r.Gate != "numbers_changed" || r.Score != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestForbiddenAndInventedServings(t *testing.T) {
	c := load(t, "two-recipes")
	r := Grade(c, "succeeded", "", out(map[string]any{"title": "きのこの炊き込みご飯", "servings": "4人分", "ingredients": []string{"米 2合", "しめじ 1パック", "白だし 大さじ3", "豚バラ肉 100g"}, "steps": []string{"米を研いで30分浸水する。", "炊く。"}, "assessment": ""}))
	if r.Status != Failed || r.Score <= 0 || r.Score >= 1 {
		t.Fatalf("%+v", r)
	}
	c = load(t, "unknown-servings")
	r = Grade(c, "succeeded", "", out(map[string]any{"title": "ほうれん草のおひたし", "servings": "2人分", "ingredients": []string{"ほうれん草 1束", "醤油 小さじ2"}, "steps": []string{"ゆでる", "切ってかける"}, "assessment": ""}))
	if r.Status != Failed {
		t.Fatalf("invented servings passed: %+v", r)
	}
}

func TestUnsucceededJobIsNotEvaluated(t *testing.T) {
	r := Grade(load(t, "basic-nikujaga"), "waiting", "quota_wait", nil)
	if r.Status != NotEvaluated || len(r.Checks) != 0 {
		t.Fatalf("%+v", r)
	}
	s := Summarize([]Result{r, {Status: Passed, Score: 1}, {Status: Failed, Score: 0.5}})
	if s.Total != 3 || s.NotEvaluated != 1 || s.MeanScore != 0.75 {
		t.Fatalf("%+v", s)
	}
}

func TestPaywallRequiresNoInventedSteps(t *testing.T) {
	c := load(t, "member-only-truncated")
	r := Grade(c, "succeeded", "", out(map[string]any{"title": "本格バターチキンカレー", "servings": "4人分", "ingredients": []string{"鶏もも肉 400g", "トマト缶 1缶", "バター 30g", "生クリーム 100ml"}, "steps": []string{}, "assessment": "作り方は会員限定のため読めませんでした"}))
	if r.Status != Passed {
		t.Fatalf("%+v", r)
	}
}
