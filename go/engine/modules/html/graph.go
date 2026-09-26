package html

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	stdhtml "html"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Matches the same :::graph ... ::: block react-markdown renders client-side via graphviz-wasm (see Markdown/components/graph.tsx).
var graphBlockRe = regexp.MustCompile(`(?s):::graph\s*\n(.*?)\n:::`)

// renderGraphBlocks extracts each :::graph block, replacing it with a
// placeholder paragraph goldmark compiles untouched; the returned map swaps
// that whole <p> back out for the rendered SVG (or an error message) after compilation.
func renderGraphBlocks(src string) (string, map[string]string) {
	i := 0
	subs := map[string]string{}
	out := graphBlockRe.ReplaceAllStringFunc(src, func(block string) string {
		dot := graphBlockRe.FindStringSubmatch(block)[1]
		token := fmt.Sprintf("GRAPH_BLOCK_PLACEHOLDER_%d", i)
		i++
		if svg, err := renderDOT(dot); err == nil {
			// An <img> data URL keeps the SVG inert: no scripts, no clickable javascript: links from DOT URL= attributes.
			subs["<p>"+token+"</p>"] = `<div class="markdown-graph" role="graph"><img alt="graph" src="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString([]byte(svg)) + `"></div>`
		} else {
			subs["<p>"+token+"</p>"] = `<div style="color:red;padding:10px;border:1px solid red"><strong>DOT Error:</strong> ` + stdhtml.EscapeString(err.Error()) + `</div>`
		}
		return "\n\n" + token + "\n\n"
	})
	return out, subs
}

// renderDOT shells out to the graphviz `dot` CLI (DOT source via stdin, no shell involved) to render SVG.
func renderDOT(dot string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dot", "-Tsvg")
	cmd.Stdin = strings.NewReader(dot)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}
	return out.String(), nil
}
