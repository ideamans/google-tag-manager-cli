package gtm

import (
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestAPIErrorSummarizesHTML(t *testing.T) {
	body := `<!DOCTYPE html><html><title>Error 404 (Not Found)!!1</title><style>*{margin:0}</style>` +
		`<p>The requested URL <code>/tagmanager/v2/x?a=1&amp;b=2</code> was not found`
	err := apiError("op", &googleapi.Error{Code: 404, Body: body})
	if strings.Contains(err.Error(), "<style>") || !strings.Contains(err.Error(), "URL /tagmanager/v2/x?a=1&b=2") {
		t.Errorf("got %q", err)
	}
	if StatusCode(err) != 404 {
		t.Errorf("status = %d", StatusCode(err))
	}
}

func TestFingerprintHint(t *testing.T) {
	err := apiError("tags.update", &googleapi.Error{Code: 400, Message: "The provided entity fingerprint is not valid."})
	if !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Errorf("no hint: %q", err)
	}
}
