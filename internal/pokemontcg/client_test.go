package pokemontcg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetCardsFollowsPagination(t *testing.T) {
	var gotKey, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		gotQuery = r.URL.Query().Get("q")
		// 3 cards in pages of 2, as if maxPageSize were 2.
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"data": [{"id": "a-1"}, {"id": "a-2"}], "totalCount": 3}`)
			return
		}
		fmt.Fprint(w, `{"data": [{"id": "a-3"}], "totalCount": 3}`)
	}))
	defer server.Close()

	client := NewClient("secret")
	client.baseURL = server.URL
	cards, err := client.GetCards(context.Background(), "a")

	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 3 || cards[2].ID != "a-3" {
		t.Errorf("cards = %+v", cards)
	}
	if gotKey != "secret" || gotQuery != "set.id:a" {
		t.Errorf("key = %q, q = %q", gotKey, gotQuery)
	}
}

func TestRetriesServerErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"data": {"id": "sv1", "ptcgoCode": "SVI"}}`)
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL
	client.retryWait = time.Millisecond
	set, err := client.GetSet(context.Background(), "sv1")

	if err != nil || set.PtcgoCode != "SVI" || calls != 3 {
		t.Errorf("set = %+v, err = %v, calls = %d", set, err, calls)
	}
}

func TestDoesNotRetryClientErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient("")
	client.baseURL = server.URL
	if _, err := client.GetSet(context.Background(), "nope"); err == nil || calls != 1 {
		t.Errorf("err = %v, calls = %d", err, calls)
	}
}
