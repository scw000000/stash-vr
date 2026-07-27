package browse

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestSceneTemplateInlineJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}

	var rendered bytes.Buffer
	if err := sceneTmpl.Execute(&rendered, SceneDetailData{DirectStreamURL: "/test.mp4"}); err != nil {
		t.Fatalf("render scene template: %v", err)
	}
	inlineScript := regexp.MustCompile(`(?s)<script>\s*(.*?)\s*</script>`).FindSubmatch(rendered.Bytes())
	if len(inlineScript) != 2 {
		t.Fatal("inline scene script not found")
	}
	path := t.TempDir() + "/browse_scene.js"
	if err := os.WriteFile(path, inlineScript[1], 0o600); err != nil {
		t.Fatalf("write rendered JavaScript: %v", err)
	}
	if output, err := exec.Command(node, "--check", path).CombinedOutput(); err != nil {
		t.Fatalf("rendered JavaScript syntax: %v\n%s", err, output)
	}
}

func TestSubtitleOptionsUseInPlaceArrowSelectors(t *testing.T) {
	var rendered bytes.Buffer
	if err := sceneTmpl.Execute(&rendered, SceneDetailData{DirectStreamURL: "/test.mp4"}); err != nil {
		t.Fatalf("render scene template: %v", err)
	}
	source := rendered.String()
	for _, expected := range []string{
		"id: prefix + '-previous'",
		"label: '<'",
		"id: prefix + '-next'",
		"label: '>'",
		"label: subtitleOptionText(field), fontScale: 0.36",
		"textColor: '#ffffff', fontScale: 0.42, bold: true",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("subtitle selector is missing %q", expected)
		}
	}

	start := strings.Index(source, "function stepSubtitleOption(field, direction)")
	if start < 0 {
		t.Fatal("stepSubtitleOption function not found")
	}
	end := strings.Index(source[start:], "function subtitleJobActive")
	if end < 0 {
		t.Fatal("could not isolate stepSubtitleOption function")
	}
	stepper := source[start : start+end]
	if strings.Contains(stepper, "renderCurvedDetailPanel") {
		t.Fatal("subtitle option stepper rebuilds the detail panel")
	}
	if !strings.Contains(stepper, "refreshSubtitleOptionRows()") {
		t.Fatal("subtitle option stepper does not update its existing rows")
	}
}

func TestDetailPanelUsesSubtitleOptionTextAsGlobalFontFloor(t *testing.T) {
	var rendered bytes.Buffer
	if err := sceneTmpl.Execute(&rendered, SceneDetailData{DirectStreamURL: "/test.mp4"}); err != nil {
		t.Fatalf("render scene template: %v", err)
	}
	source := rendered.String()
	for _, expected := range []string{
		"const DETAIL_MIN_FONT_SCALE = 0.36",
		"Math.max(DETAIL_MIN_FONT_SCALE, requestedFontScale)",
		"Math.ceil(canvasH * DETAIL_MIN_FONT_SCALE)",
		"Math.max(minimumFontPx, Math.floor(fontPx * 0.88))",
		"parent.dataset.detailFontScale = (fontPx / canvasH).toFixed(4)",
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("detail typography floor is missing %q", expected)
		}
	}
}
