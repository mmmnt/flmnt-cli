package intent

import (
	"reflect"
	"testing"
)

/*
The founder writes numbered lists constantly — "1. includes 2. is there a question here? 3. create the
query we need" is three separate intents in one message, and splitting on terminal punctuation alone
keeps them as one. Splitting on the markers too promotes 20 bare markers ("2.", "3.") to sentences
across the real corpus, which are not sentences and must not be labelled.
*/
func TestSentencesSplitsOnTerminalPunctuationAndListMarkers(t *testing.T) {
	got := Sentences("1. includes 2. is there a question here? 3. create the query we need")

	want := []string{"1. includes", "2. is there a question here?", "3. create the query we need"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

// A marker is carried into its sentence — it is part of what was written, and the labeller strips it
// before reading the first word. A marker with nothing after it carries no intent and is dropped.
func TestSentencesDropsABareListMarker(t *testing.T) {
	got := Sentences("2. 3. leave it.")

	want := []string{"3. leave it."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}
