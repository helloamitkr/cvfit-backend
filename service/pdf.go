package service

import (
	"context"
	"encoding/base64"
	"strings"

	cvpdf "github.com/helloamitkr/cvfit-tools/pdf"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

// Watermark assets — built once at startup.
var (
	wmB64    = buildWMBase64()
	wmHTML   = buildWMHTML(wmB64)
	wmScript = buildWMScript(wmB64)
)

func buildWMBase64() string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="350" height="350" viewBox="0 0 350 350">` +
		`<text x="175" y="155" text-anchor="middle" dominant-baseline="middle"` +
		` font-family="Arial,Helvetica,sans-serif" font-size="48" font-weight="900"` +
		` fill="#000000" fill-opacity="0.28" transform="rotate(-45 175 175)" letter-spacing="6">PREVIEW</text>` +
		`<text x="175" y="205" text-anchor="middle" dominant-baseline="middle"` +
		` font-family="Arial,Helvetica,sans-serif" font-size="14" font-weight="700"` +
		` fill="#000000" fill-opacity="0.20" transform="rotate(-45 175 175)" letter-spacing="5">ZustResume</text>` +
		`</svg>`
	return base64.StdEncoding.EncodeToString([]byte(svg))
}

func buildWMHTML(b64 string) string {
	return `<div id="zr-wm" style="position:fixed;top:0;left:0;width:100%;height:100%;` +
		`z-index:2147483647;pointer-events:none;` +
		`background-image:url('data:image/svg+xml;base64,` + b64 + `');` +
		`background-size:350px 350px;background-repeat:repeat;` +
		`-webkit-print-color-adjust:exact;print-color-adjust:exact;"></div>`
}

func buildWMScript(b64 string) string {
	return `(function(){` +
		`if(document.getElementById('zr-wm'))return;` +
		`var e=document.createElement('div');` +
		`e.id='zr-wm';` +
		`e.style.position='fixed';` +
		`e.style.top='0';` +
		`e.style.left='0';` +
		`e.style.width='100%';` +
		`e.style.height='100%';` +
		`e.style.zIndex='2147483647';` +
		`e.style.pointerEvents='none';` +
		`e.style.backgroundImage='url("data:image/svg+xml;base64,` + b64 + `")';` +
		`e.style.backgroundSize='350px 350px';` +
		`e.style.backgroundRepeat='repeat';` +
		`e.style.webkitPrintColorAdjust='exact';` +
		`e.style.printColorAdjust='exact';` +
		`document.body.appendChild(e);` +
		`})()`
}

// GeneratePDF renders the resume to HTML then prints it to PDF via headless Chrome.
// If hasPaid is false, a tiled "PREVIEW / ZustResume" watermark is applied.
func (s *Service) GeneratePDF(ctx context.Context, r *resume.Resume, hasPaid bool) ([]byte, *apperrors.AppError) {
	html, appErr := s.RenderResume(ctx, r)
	if appErr != nil {
		return nil, appErr
	}

	script := ""
	if !hasPaid {
		// Inject watermark into HTML source and also via JS (belt-and-suspenders).
		html = strings.Replace(html, "</body>", wmHTML+"</body>", 1)
		script = wmScript
	}

	buf, err := cvpdf.FromEnv().HTMLToPDF(html, script)
	if err != nil {
		return nil, apperrors.InternalServerWrap("pdf generation failed", err)
	}
	return buf, nil
}
