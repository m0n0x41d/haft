package authority

import "testing"

func TestPreparedManualSpeechActSealsIntentReviewAndDigest(t *testing.T) {
	profileIntent, _ := testPreparedAuthorityIntent(t)
	intent, ok := profileIntent.SpeechActIntent()
	if !ok {
		t.Fatal("profile fixture omitted its generic SpeechAct intent")
	}
	reviewText := "Bind the reviewed project-profile declaration."
	prepared, err := PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct: %v", err)
	}
	gotIntent, intentOK := prepared.Intent()
	gotText, textOK := prepared.ReviewText()
	gotDigest, digestOK := prepared.ReviewDigest()
	if !intentOK || !textOK || !digestOK {
		t.Fatal("prepared manual SpeechAct omitted a sealed binding")
	}
	wantIntentDigest, _ := intent.Digest()
	gotIntentDigest, _ := gotIntent.Digest()
	if gotIntentDigest != wantIntentDigest || gotText != reviewText {
		t.Fatal("prepared manual SpeechAct changed its intent or review text")
	}
	wantDigest, err := SpeechActIntentReviewDigest(intent, reviewText)
	if err != nil {
		t.Fatalf("SpeechActIntentReviewDigest: %v", err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("review digest = %s, want %s", gotDigest.String(), wantDigest.String())
	}
	different, err := PrepareManualSpeechAct(intent, reviewText+" Changed.")
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct changed review: %v", err)
	}
	differentDigest, _ := different.ReviewDigest()
	if differentDigest == gotDigest {
		t.Fatal("different review text retained the same sealed digest")
	}
}

func TestLiteralSpeechActRuleAcceptsDomainOwnedCanonicalVerb(t *testing.T) {
	rule, err := NewLiteralSpeechActUtteranceRule("BIND-DECISION", "CHECKOUT ARCHITECTURE")
	if err != nil {
		t.Fatalf("NewLiteralSpeechActUtteranceRule: %v", err)
	}
	if !rule.valid() {
		t.Fatal("domain-owned canonical verb produced an invalid utterance rule")
	}
	for _, invalid := range []string{"bind", "BIND DECISION", "-BIND"} {
		_, err := NewLiteralSpeechActUtteranceRule(invalid, "CHECKOUT ARCHITECTURE")
		if err == nil {
			t.Fatalf("invalid verb %q was accepted", invalid)
		}
	}
}
