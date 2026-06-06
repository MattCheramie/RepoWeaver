package visual

import (
	"strings"
	"testing"
)

func TestHeroDeterministicAndSelfContained(t *testing.T) {
	spec := HeroSpec{
		Title:  "How RepoWeaver Clusters Repository History",
		Format: "deep_dive",
		Tags:   []string{"go", "llm", "seo"},
		Repo:   "mattcheramie/repoweaver",
	}
	a := Hero(spec)
	b := Hero(spec)
	if a != b {
		t.Fatal("Hero is not deterministic for identical input")
	}
	if !strings.HasPrefix(a, "<svg") || !strings.HasSuffix(a, "</svg>") {
		t.Fatalf("Hero did not return a single <svg> element: %.40s…", a)
	}
	// Self-contained: no external resource references that would require hosting.
	// (The required xmlns="http://www.w3.org/2000/svg" namespace is not a fetch.)
	for _, bad := range []string{"<image", "xlink:href", `href="http`, "url(http"} {
		if strings.Contains(a, bad) {
			t.Errorf("Hero SVG contains external reference %q", bad)
		}
	}
	if !strings.Contains(a, "mattcheramie/repoweaver") {
		t.Error("Hero SVG missing repo footer")
	}
}

func TestHeroVariesWithTitle(t *testing.T) {
	one := Hero(HeroSpec{Title: "Alpha", Format: "blog"})
	two := Hero(HeroSpec{Title: "Beta", Format: "blog"})
	if one == two {
		t.Error("Hero should differ for different titles (seeded layout)")
	}
}

func TestHeroEscapesTitle(t *testing.T) {
	h := Hero(HeroSpec{Title: `<script>"x"&y`, Format: "blog"})
	if strings.Contains(h, "<script>") {
		t.Error("Hero did not escape angle brackets in the title")
	}
}

func TestChartTypesRender(t *testing.T) {
	data := []DataPoint{{"PRs", 12}, {"Issues", 7}, {"Docs", 4}}
	for _, typ := range []string{"bar", "line", "area", "pie", "", "wat"} {
		svg, err := Chart(ChartSpec{Type: typ, Title: "Activity", Data: data})
		if err != nil {
			t.Fatalf("Chart(%q) returned error: %v", typ, err)
		}
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
			t.Errorf("Chart(%q) is not a single svg element", typ)
		}
		if !strings.Contains(svg, "Activity") {
			t.Errorf("Chart(%q) missing title", typ)
		}
	}
}

func TestChartDeterministic(t *testing.T) {
	cs := ChartSpec{Type: "bar", Title: "T", Data: []DataPoint{{"a", 1}, {"b", 2}}}
	if Chart1, _ := Chart(cs); true {
		Chart2, _ := Chart(cs)
		if Chart1 != Chart2 {
			t.Error("Chart is not deterministic")
		}
	}
}

func TestParseChart(t *testing.T) {
	good := `{"type":"bar","title":"x","data":[{"label":"a","value":3}]}`
	cs, err := ParseChart(good)
	if err != nil {
		t.Fatalf("ParseChart valid spec failed: %v", err)
	}
	if cs.Type != "bar" || len(cs.Data) != 1 || cs.Data[0].Value != 3 {
		t.Errorf("ParseChart decoded unexpected spec: %+v", cs)
	}

	for _, bad := range []string{`not json`, `{"type":"bar","data":[]}`, `{"type":"bar"}`} {
		if _, err := ParseChart(bad); err == nil {
			t.Errorf("ParseChart(%q) should have errored", bad)
		}
	}
}

func TestChartEmptyDataErrors(t *testing.T) {
	if _, err := Chart(ChartSpec{Type: "bar"}); err == nil {
		t.Error("Chart with no data should error")
	}
}

func TestNiceMax(t *testing.T) {
	cases := map[float64]float64{0.4: 0.5, 3: 5, 7: 10, 12: 20, 0: 1, 95: 100}
	for in, want := range cases {
		if got := niceMax(in); got != want {
			t.Errorf("niceMax(%v) = %v, want %v", in, got, want)
		}
	}
}
